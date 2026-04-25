# Handover: N03 Step 1+2 完了 / Step 3-8 継続

**最終更新**: 2026-04-25 14:42 JST  
**対象セッション**: `/devflow:implement` で Phase N の N03（find 層ゼロベース再設計の実装初動）に着手  
**停止理由**: ユーザー判断（トークン制約、キリよく終了の方針）

## 完了済み（このセッション）

### Step 1: PoC + retry instrumentation + PoC レポート

| ファイル | 種別 | 内容 |
|---|---|---|
| `internal/boardapi/retry.go` | 更新 | `recordRetryAttempt(req.Context())` 呼び出しを `DoWithRetry`/`DoWithRetryFull` のリトライループ確定後に 2 行追加（behavior change なし） |
| `internal/boardapi/retry_instrumentation.go` | 新規 | `WithRetryCounter(parent) (context.Context, *atomic.Int32)` + `recordRetryAttempt(ctx)`（ctx.Value 経由のテスト用 instrumentation、advisor #2 推奨手段 (a)） |
| `internal/boardapi/e2e_find_layer_poc_test.go` | 新規 | 4 種の PoC scale test（estimate/order/delivery/receipt）+ pre-flight rate limit budget check |
| `plans/board-phase-n-m02-document-poc-report.md` | 新規 | PoC レポート（`status: Ready`、§0-§5 確定、§6 placeholder） |

### Step 2: go.mod 更新

| ファイル | 種別 | 内容 |
|---|---|---|
| `go.mod` / `go.sum` | 更新 | `golang.org/x/sync v0.19.0 → v0.20.0`（indirect、Step 4 で reverse_map.go が `singleflight` を import すると direct 昇格） |

### 実 API scale 実測（2026-04-25）

| RG | cold_latency | pages | items | retry | 判定 |
|---|---|---|---|---|---|
| estimate | 19465 ms | 25 | 2425 | 0 | 有効 |
| order | 16466 ms | 25 | 2425 | 0 | 有効（mise→go test 直接呼び切替で retry 解消） |
| delivery | 16930 ms | 25 | 2425 | 0 | 有効（receipt 完走後 bucket 回復で retry 解消） |
| receipt | 31915 ms | 25 | 2425 | 0 | 有効 |

**全 4 種 retry=0 達成 → 案 A 確定（gate 通過）**。  
ただし全 4 種 cold_latency >10s 観測 → N02 §5.3「10 秒超過時 ctx timeout で中断、ProjectID=0 フォールバック」のトリガ発動。

### 副次 commit 候補（collateral）

| ファイル | 内容 |
|---|---|
| `internal/boardapi/e2e_accounting_types_test.go` | 旧 signature → 新 signature 修復（XxxRaw(ctx) → XxxRaw(ctx, opts), 戻り値 2→3） |
| `internal/boardapi/e2e_document_send_channels_test.go` | 同上 |
| `internal/boardapi/e2e_groups_test.go` | 同上 |
| `internal/boardapi/e2e_payment_terms_test.go` | 同上 |
| `internal/boardapi/e2e_project_types_test.go` | 同上 |
| `internal/boardapi/e2e_purchase_types_test.go` | 同上 |
| `internal/boardapi/e2e_users_test.go` | 同上 |

これらは PoC 実 API テスト実行時に build error で発覚した broken e2e_*_test.go の修復。**user 承認済**で chore commit 分離。

### ロードマップ更新

| ファイル | 内容 |
|---|---|
| `plans/board-phase-n-roadmap.md` | Current Focus を N03 (Step 1+2 完了、Step 3-8 残り) に更新、Progress に Step 1/2 [x] 追記、Changelog + Next Action 更新 |

### untracked（このセッション以前から）

| ファイル | 推奨対応 |
|---|---|
| `AGENTS.md` | 別件（このセッション関連なし）。次セッション以降で扱う |
| `plans/witty-sauteeing-kurzweil.md` | N03 計画書、本 commit 群に含める |

## 残作業（次セッション、`/devflow:implement` 再開で着手）

### N03 Step 3-8 + review + 最終 commit

詳細は `plans/witty-sauteeing-kurzweil.md` の Step 3-8 + Verification を参照。

#### Step 3: find2 パッケージ骨格

- `internal/service/find2/service.go`
  - 11 個の repo interface（**FindGroup 関連は削除**）
  - `Repos struct`（15 リソース、Groups 除外）
  - `Service struct`（小文字フィールド + `reverseMappers map[string]*reverseMapper`）
  - `New(r Repos) *Service`（reverseMappers の make のみ）
- `internal/service/find2/types.go`
  - `FindCommonOpts struct { Limit int; Opts repository.ReadOptions }` + `validate() error`（Limit<0 で error、Limit=0 OK）
  - `validatable` interface + `validateQuery(common, specific)` ヘルパー
  - 11 Query（FindGroupQuery 削除、各 Query は FindCommonOpts embed + validate() 実装）
  - 11 Result（Document 4 種は `ProjectID int` / `ClientID int` 追加、JSON tag）

#### Step 4: 共通ヘルパー実装（**ctx timeout フォールバック追加実装が必須**）

- `text_match.go`: 旧 find/text_match.go から containsText/derefString/projectClientID(Ptr) を完全移植
- `filter.go`: `filterByStatuses[T]`, `filterByStatus[T]` ジェネリクス
- `resolver.go`: `resolveClientAndProject` / `resolveVendorAndProject` errgroup 並列、enrichment 失敗で slog.Warn + swallow
- `reverse_map.go`:
  - `reverseMapper` struct (sync.Map + singleflight.Group + buildTimeout time.Duration)
  - `newReverseMapper(p, group)` (buildTimeout=10*time.Second 既定)
  - `newReverseMapperWithTimeout(p, group, timeout)` (test 用)
  - `Lookup(ctx, docID, opts) (projectID int, ok bool, err error)`
  - **ctx timeout フォールバック**: ensureBuilt 内で `context.WithTimeout(ctx, m.buildTimeout)`、deadline 発火時は `slog.Warn` + Lookup `(0, false, nil)` 返却
  - **cold-reverse-map ログ**: build 開始時 `slog.Warn("[SLOW:cold-reverse-map] projects RG=<g> building")` を stderr 出力
  - extractDocIDs: estimate/order/delivery/receipt の closure を responseGroup 引数で振り分け

#### Step 5: helpers_test.go + ヘルパー unit test

- T01-T24（24 ケース）+ timeout フォールバック test
- `go test -race -count=1 ./internal/service/find2/` で race なし pass
- testify 不使用、内部 test (`package find2`) で実装、旧 find/helpers_test.go の stub struct パターン踏襲

#### Step 6: app.go に FindService2() 暫定追加

- `internal/app/app.go` に `FindService2() *find2.Service` 追加（既存 `FindService()` は無変更）
- import に `find2 "github.com/youyo/board/internal/service/find2"` 追加

#### Step 7: mise.toml に test:race タスク

```toml
[tasks."test:race"]
description = "Run unit tests with race detector (excludes e2e)"
run = "go test -race -count=1 ./..."
```

#### Step 8: N07b rename drill

- `git grep -n 'find2\.' | wc -l`、`git grep -n 'FindService2'`、`git grep -l 'service/find2'` で影響範囲調査
- `plans/board-phase-n-m02-document-poc-report.md` §6 placeholder を結果で置換
- 実 rename はせず drill のみ

#### code-reviewer + commit-agent

- code-reviewer（Opus）に独立レビュー（実装コンテキストは渡さない）
- 修正 → commit-agent（Haiku）で論理単位ごとに Conventional Commits（日本語）で commit

## 重要な注意事項（次セッション開始時に必ず確認）

1. **`mise run` を直接呼ばない**: sandbox で TLS 証明書検証エラーになる。`go test`/`go run` を直接呼ぶ。memory `feedback_use_go_test_directly.md` に記録済
2. **`reverse_map.go` に ctx timeout フォールバックを追加実装**: user 承認済スコープ拡張、PoC レポート §3/§4 で trigger 確定（cold_latency >10s）
3. **旧 `internal/service/find/` は完全無変更**: `git diff internal/service/find/` empty が verification 項目
4. **旧 e2e build は壊れていた**: 7 ファイルの signature 修復は本 commit に含めて修復済。今後の e2e build は OK
5. **Document Entity に project_id/client_id を追加しない**: 案 A 採用（dump 8 件 + scale 4 種双方の根拠で確定）
6. **Sprint Contract**: build pass + lint/型 pass + test pass + race pass + ドキュメント更新（PoC report §6）

## 検証コマンド（再開時の sanity check）

```bash
# build 健全性
go build ./...
go vet ./...
go test -count=1 ./...

# E2E PoC（必要なら再実行、retry 発動時は前テストとの間隔を 1 分以上空ける）
go test -tags e2e -v -count=1 -run TestE2E_FindLayerPoC_Projects_RG_Estimate_ScaleCold ./internal/boardapi/

# scope 維持確認
git diff internal/service/find/    # empty 期待
git status --short                  # 残りは N03 Step 3-8 のファイルのみ期待
```
