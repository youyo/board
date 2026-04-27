# board cache redesign — LLM-driven freshness model

ステータス: **draft（実装前のレビュー待ち）**

board MCP / CLI のキャッシュ動作を「daily auto-refresh の暗黙更新」モデルから「LLM が `cached_at` を見て明示 `--refresh` を指示する」モデルへ転換する。Lambda + S3 Files 上での動作を含めた現実的な共有キャッシュ運用を実現する。

旧 `plans/board-dynamodb-cache-design.md` は本 doc で置き換え（DynamoDB 路線は据置）。

## 1. ゴール

- LLM がキャッシュの新鮮度を `cache[].cached_at` から判断できる
- Refresh は明示指示のみ（`--refresh` = delta、`--refresh-full` = full）
- Refresh の同時実行は **TryLock + 即 429** で wait させない
- Lambda マルチコンテナで cache を共有可能（**S3 Files + SQLite** ルート）
- find 層が cache を活用して BOARD API rate limit を温存

## 2. 主要決定（確定済）

| 項目 | 決定 |
|---|---|
| Daily auto-refresh | **廃止** |
| 初回 auto-build | **廃止**（初回は user が `--refresh-full` を実行） |
| `--refresh` フラグ | **delta** 取得 (`UpdatedAtGteq=cursor`) |
| `--refresh-full` フラグ | **full** 取得（`DeleteAll + 全件再取得`）|
| `--force-refresh` | **削除**（alias 残さない） |
| MCP `refresh` 引数 | enum: `"none" \| "delta" \| "full"` |
| レスポンスフィールド名 | `cache` (配列) |
| 配列要素 | `{resource, cached_at, full_refreshed_at}` |
| Refresh 排他 | TryLock 失敗 → 即 429 + `retry_after_seconds`（固定 30） |
| 429 表現（MCP） | `IsError=true` + `error_code: "refresh_in_progress"` |
| cache-first filter 正規化 | NFKC + ToLower + TrimSpace（実 API drift test 付き） |
| ストレージ | SQLite（local CLI、`/tmp` Lambda、S3 Files マウント） |
| DynamoDB cache backend | **据置**（必要になったら別途検討） |

## 3. レスポンス形式

```json
{
  "items": [...],
  "cache": [
    {
      "resource": "clients",
      "cached_at": "2026-04-25T10:00:00Z",
      "full_refreshed_at": "2026-04-01T12:00:00Z"
    },
    {
      "resource": "projects",
      "cached_at": "2026-04-26T08:30:00Z",
      "full_refreshed_at": "2026-04-01T12:00:00Z"
    }
  ]
}
```

- `cached_at` = 最終 sync (`last_synced_at`)
- `full_refreshed_at` = 最終 full refresh (`last_full_synced_at`)
- 配列要素は find が実際に read したリソースぶんだけ

LLM 判断材料:
- `cached_at` 古い → `--refresh` で delta
- `full_refreshed_at` から N 日経過 → `--refresh-full` で削除 entity も洗う

## 4. cache-first filter（重要）

### スコープ
| Filter | サポート | 備考 |
|---|---|---|
| `ID` (`GetByID`) | ✅ 既存 | cache hit |
| `NameCont` | ✅ 新規 | NFKC + ToLower + TrimSpace で Go filter |
| `ClientIDEq` / `VendorIDEq` | ✅ 新規 | 数値完全一致 |
| `StatusEq` | ✅ 新規 | 文字列完全一致 |
| `UpdatedAtGteq` / `UpdatedAtLteq` | ✅ 新規 | 日時比較 |
| `ResponseGroup` | ❌ 不可 | embedded entity を再構築できない（後述） |
| `Tags` | ❌ 不可 | 配列フィルタ複雑、利用箇所も少ない |
| `Page` / `PerPage` | N/A | cache は全件持つ、不要 |

### ResponseGroup 制限

projects.Search に `ResponseGroup="estimate"` 等を付けると API は埋め込み entity を返すが、cache は flat に保存しているため再現できない。document fanout（estimate/order/delivery/receipt の ClientName + ProjectName 経路）は **引き続き API 直叩き**で動作する。

ただし fanout 内の `GetByDocumentID(doc_id)` は既存通り cache hit するため、cold start 後の再 query では ID lookup ぶん（最大 50 calls）が削減される。

将来の改善余地: cached documents から projects→document_ids の inverted index を Go で計算すれば fanout も cache-first 化可能（**Phase 2**、今回は対象外）。

### 名前 fold ヘルパー

```go
// internal/cache/fold/fold.go
package fold

import (
    "strings"
    "golang.org/x/text/unicode/norm"
)

// NameFold は BOARD API の name_cont 挙動に近づけた正規化を返す。
// NFKC + ToLower + TrimSpace。漢字↔カナ fold は行わない。
func NameFold(s string) string {
    return strings.ToLower(norm.NFKC.String(strings.TrimSpace(s)))
}

func Contains(haystack, needle string) bool {
    n := NameFold(needle)
    if n == "" {
        return true
    }
    return strings.Contains(NameFold(haystack), n)
}
```

### Drift test

`internal/repository/cache_filter_drift_e2e_test.go` (build tag `e2e`) で、実 API 結果と Go filter 結果を比較:

```go
cases := []string{"COI", "coi", "ＣＯＩ", "株", "会社", " COI "}
for _, q := range cases {
    apiHits := api.ListClients(ctx, ClientListOptions{NameCont: q})
    cacheHits := goFilter(allCachedClients, q)
    diff := compareIDs(apiHits, cacheHits)
    if len(diff) > 0 {
        t.Errorf("drift for %q: %v", q, diff)
    }
}
```

PR マージ前に実 API で実行して drift ゼロを確認。

## 5. TryLock + 429

### 仕様
```go
// internal/refresh/lock.go
type LockManager interface {
    // ... 既存 WithLock 等

    // TryLock は cache refresh の排他ロックを試行する。
    // 成功時: acquired=true, release を呼ぶ責任は呼び出し元
    // 既に保持されている: acquired=false, holder & elapsed を返す
    // staleLockTimeout (5min) 超過時は takeover で acquired=true
    TryLock(ctx context.Context, profile, resource, owner string) (
        acquired bool, holder string, elapsed time.Duration, err error)
}
```

### エラー型

```go
// internal/repository/errors.go
type RefreshInProgressError struct {
    Resource          string
    Holder            string
    ElapsedSeconds    int
    RetryAfterSeconds int
}

func (e *RefreshInProgressError) Error() string {
    return fmt.Sprintf("refresh in progress for %s (holder=%s, elapsed=%ds)", e.Resource, e.Holder, e.ElapsedSeconds)
}
```

### MCP / CLI 出力
```json
{
  "error": true,
  "error_code": "refresh_in_progress",
  "message": "refresh in progress for clients (holder=lambda-12, elapsed=3s)",
  "retry_after_seconds": 30
}
```

CLI: exit code 75 (EX_TEMPFAIL 相当) + stderr に同 JSON。

## 6. Refresh 仕様

### find に refresh を指定するパス

| 経路 | 指定方法 |
|---|---|
| CLI find | `--refresh` / `--refresh-full` |
| MCP find | `refresh: "delta"` / `"full"` (default `"none"`) |

### 影響範囲

`--refresh` / `refresh: delta` は **find が touch する全 resource** に対して delta を実行（fanout なら clients + projects + estimates 等）。各 resource に対して TryLock を取り、1 つでも失敗したら 429 即返却。

### 削除 entity

delta は updated_at 基準なので削除を検知できない。LLM が `full_refreshed_at` を見て古ければ `--refresh-full` を指示する想定。

## 7. ストレージ

### Lambda 用パス
```
BOARD_CACHE_DIR=/tmp/board-cache             # warm reuse のみ、cold start で消える
BOARD_CACHE_DIR=/mnt/s3/board-cache          # S3 Files マウント (永続+共有)
BOARD_CACHE_DIR=/mnt/efs/board-cache         # EFS (永続+共有)
```

`config.toml` の `[profile.X] cache_dir = "..."` でも上書き可能（環境変数優先）。

### 既存パス
`~/.cache/board/cache.db` は変更なし。

## 8. config.toml 変更

### 削除
```toml
[profile.default]
daily_auto_refresh = true   # ← 削除
```

### 追加
```toml
[profile.default]
cache_dir = "/tmp/board-cache"   # 任意 (env: BOARD_CACHE_DIR)
```

### Migration
- 既存 `daily_auto_refresh` は **設定読み込み時に warning + ignore**
- v0.11.0 で alias 削除

## 9. 実装計画（commit 順）

advisor 推奨順を採用:

| # | 名称 | 内容 | 工数 |
|---|---|---|---|
| C1 | CacheInfo + cache-first filter | `internal/cache/fold/`、`CacheInfo` 型、`FindResult[T]` 化、Repository.List の cache-first 化、find 層が touched resources を集約、drift test | L |
| C2 | refresh enum + flag | `--refresh` / `--refresh-full` CLI、MCP `refresh` enum、`ReadOptions.RefreshMode` | M |
| C3 | TryLock + 429 | `LockManager.TryLock`、`RefreshInProgressError`、CLI/MCP 出力 | M |
| C4 | daily/初回 auto-build 廃止 | `policy.NeedsDailyRefresh` 削除、`maybeRefresh` 改修、`DailyAutoRefresh` 設定削除、初回 ForceRefresh 自動実行削除 | M |
| C5 | docs / examples | CLAUDE.md / README.md / examples/lambroll/ 更新、`BOARD_CACHE_DIR` env、S3 Files マウント手順追記 | S |

総工数: **~10-14 営業日**

### C1 の内訳（最大塊）
- `cache/fold/` パッケージ + unit test
- `repository/cache_filter.go` ヘルパー (filter on cached entities)
- `repository/result.go` `FindResult[T]` / `CacheInfo`
- 22 リポジトリの List 改修（zero filter / non-zero filter どちらも cache-first 化、ResponseGroup を含む場合のみ bypass）
- find 層 11 メソッドの戻り値型変更
- CLI 11 コマンドの出力形式変更
- MCP 11 tool の出力形式変更
- drift test (e2e build tag)

## 10. リスクとオープン質問

### R1: ResponseGroup を含むクエリは引き続き rate limit 圧迫
fanout 1 回 = 1 (clients) + 5 (projects ResponseGroup) = **6 calls / 2 秒**。clients が cache-first 化すれば 5 calls。docs は cache hit。Phase 2 で fanout も cache-first 化可能。

### R2: drift test の運用
e2e build tag なので CI default で走らない。リリース前に手動実行する運用。

### R3: 既存コードベースへの侵襲
22 リポジトリ × 11 find メソッドの戻り値変更で diff が膨大。test 修正含めると 50+ ファイル。

### R4: Refresh が長時間化したときの 429 の妥当性
S3 Files 経由の delta refresh は数秒〜数十秒見込み。30s retry_after_seconds は短すぎるか? 計測して調整。

### R5: TryLock の atomic 性
SQLite ファイルの先勝ち lock は atomic だが、S3 Files マウントだと NFS lease lock 経由になる。挙動を実測必要。

## 11. 未確定（実装着手前に確定したい）

- [ ] `cache_dir` 設定の precedence: env > config > default で OK?
- [ ] 削除する `daily_auto_refresh` の deprecation 期間: v0.11.0 でいきなり削除 or 1 リリース挟む?
- [ ] `cache` レスポンス配列が空のとき (どの resource も cache 経由してない、稀ケース) の扱い: 空配列 / フィールド省略?
- [ ] MCP の `refresh` enum default: `"none"` / 不在で `"none"`?
- [ ] CLI で `--refresh` と `--refresh-full` を同時指定したらエラー / `--refresh-full` 優先?

## 12. 別案・採用しない設計

- **DynamoDB native cache**: 据置（前 doc の D01-D09 はそのまま参照可、必要時に着手）
- **Phase 2: project→document inverted index** in Go: fanout も完全 cache-first 化、今回スコープ外
- **Auto-refresh interval (e.g. 6h, 24h) を残す**: ユーザーは LLM-driven 路線を選択、自動 trigger は廃止
