# E2E Tests: boardapi + find サービス層

## Context

環境変数 `BOARD_API_KEY` / `BOARD_API_TOKEN` で本番 BOARD API に接続可能。
参照系操作に限定した E2E テストを、以下の 2 層で実施する：

1. **boardapi 層**（実装済み）: HTTP クライアントの API 接続性検証
2. **service/find 層**（新規）: 高レベル検索の cross-resource 解決を検証

## API コマンドの動作について（調査結果）

### `--limit` の動作
- デフォルト `--limit 50` で結果を切り詰め
- `--limit 0` で無制限取得可能
- **ページネーションカーソル（next_point 等）は出力されない** → **今後の改修課題**
- `boardapi` 層は内部で全ページ取得（100件/ページで API を叩く）するが、CLI の `--limit` で最終出力を制限
- 50件を超えるデータは `--limit 0` で全件取得可能だが、offset/cursor 方式の逐次取得は未実装
- **TODO**: `--offset` や cursor ベースのページネーションを今後追加する（メモリに記録済み）

### list/get/search の動作
- E2E テスト結果から確認済み:
  - `ListClients`: 299件 ✅
  - `GetClient(ID)`: ID指定で取得 ✅
  - `SearchClients(params)`: パラメータ検索 ✅
  - `ListUsers`: 26件 ✅（ただし `GetUser(ID)` は BOARD API が 404 を返す）
  - `ListProjects`: 2391件 ✅
  - `ListInvoices`: 11688件 ✅（大量データのため約83秒）

---

## find 層 E2E テスト設計

### テスト層: `service/find` パッケージ

- `app.New("")` で完全な DI コンテナを構築
- `app.Repos` → `find.Repos` に変換して `find.New(repos)` で find サービスを構築
- 実 API 経由で cross-resource 解決を検証

### テスト対象: 代表 4 カテゴリ

| Category | Find | Query例 | 検証内容 |
|----------|------|---------|----------|
| Simple | `FindClient` | Name 検索 | Client + Branches + Contacts 解決 |
| Simple | `FindUser` | Name 検索 | User 取得 |
| Document | `FindProject` | ID or Name | Project + Client 解決 |
| Document | `FindInvoice` | ID | Invoice + Client + Project 解決 |

API 呼び出し数: 1回のフル実行で約 10-15 calls

### ファイル配置

```
internal/service/find/
  e2e_helpers_test.go   # //go:build e2e — app.New() ベースのセットアップ
  e2e_test.go           # //go:build e2e — find E2E テストケース
```

### Step 1: `internal/service/find/e2e_helpers_test.go` (new)

```go
//go:build e2e

package find_test
```

- `skipIfNoCredentials(t)` — `BOARD_API_KEY` / `BOARD_API_TOKEN` 未設定時 `t.Skip`
- `newE2EFindService(t) *find.Service` — `app.New("")` → `find.Repos` 変換 → `find.New(repos)`
  - temp dir に最小限の config.toml を書き込み
  - `BOARD_CONFIG_PATH` / `BOARD_CACHE_PATH` を `t.Setenv`
  - `BOARD_API_KEY` / `BOARD_API_TOKEN` は環境変数から自動取得
  - `t.Cleanup` で `app.Close()` を呼ぶ

### Step 2: `internal/service/find/e2e_test.go` (new)

```go
//go:build e2e

package find_test
```

**テストケース一覧（6 テスト）:**

| Test | Method | Query | Assertions |
|------|--------|-------|------------|
| `TestE2E_FindClient_ByName` | FindClient | Name (先頭Client名) | no error, len >= 1, Client.Name 非空 |
| `TestE2E_FindUser_ByName` | FindUser | Name (先頭User名) | no error, len >= 1 |
| `TestE2E_FindProject_ByID` | FindProject | ID (先頭Project ID) | no error, len == 1, Client 解決済み |
| `TestE2E_FindInvoice_ByID` | FindInvoice | ID (先頭Invoice ID) | no error, len == 1, Client/Project 解決済み |
| `TestE2E_FindClient_ByText` | FindClient | Text 検索 | no error (テキストマッチ動作確認) |
| `TestE2E_FindProject_ByClientName` | FindProject | ClientName | no error, cross-resource 解決確認 |

**テストパターン:**
- まず低レベル `boardapi.Client` で List して先頭エンティティの ID/Name を取得
- その値を find Query に渡して find サービスを呼び出し
- 結果の構造体フィールド（関連エンティティ解決）を検証
- 404 は `skipIfNotFound` で graceful skip

### Step 3: `mise.toml` の `test:e2e` タスク更新

```toml
[tasks."test:e2e"]
description = "Run E2E tests against the real BOARD API (requires BOARD_API_KEY and BOARD_API_TOKEN)"
run = "go test -tags e2e -v -count=1 ./internal/boardapi/ ./internal/service/find/"
```

## Critical Files

| File | Action |
|------|--------|
| `internal/service/find/e2e_helpers_test.go` | **新規作成** |
| `internal/service/find/e2e_test.go` | **新規作成** |
| `mise.toml` | **修正** — find パッケージを追加 |
| `internal/service/find/service.go` | 参照 — Service, Repos, New() |
| `internal/service/find/types.go` | 参照 — Query/Result 型 |
| `internal/cli/find.go` | 参照 — findServiceFromCmd() の Repos 変換パターン |
| `internal/app/app.go` | 参照 — App.Repos の構造 |
| `internal/app/container.go` | 参照 — Repositories 構造体 |

## Verification

```bash
# 全 E2E テスト実行（boardapi + find）
mise run test:e2e

# find E2E のみ実行
go test -tags e2e -v -count=1 ./internal/service/find/ -run TestE2E

# 既存ユニットテストに影響なし確認
mise run test
```
