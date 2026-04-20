# M18: estimates Get 厳格突合（Phase G 2 件目）

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | M18 / Phase G 2 件目 |
| ロードマップ | plans/board-compliance-roadmap.md |
| 対象 | `internal/boardapi/estimates.go`（GetEstimateRaw 追加）、`internal/boardapi/e2e_estimates_test.go`（新規）、`internal/boardapi/e2e_test.go`（旧テスト削除） |
| スコープ | `EstimateEntity`（11 フィールド）の厳格フィールド突合。`GetEstimateRaw` を追加し、M17 で確立した `findAnyDocumentID` helper 経由で documentID を discovery し E2E Get テストを実施。 |
| 見積 | ~5 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetEstimateRaw 1 = 3 req 期待）|
| 前提 | M17 `findAnyDocumentID(t, client, docType)` helper が `e2e_helpers_test.go` に実装済み |

## 背景

M17 で `findAnyDocumentID(t, client, docType)` helper を確立し、document 系エンドポイントの E2E Get テスト基盤が整備された（Phase G 開始）。M18 では estimates を最初の適用対象として:

1. `GetEstimateRaw` を `estimates.go` に追加（vendors.go の `GetVendorRaw` と同パターン）
2. `e2e_estimates_test.go` を新規作成し M17 helper 経由の厳格突合 E2E テストを追加
3. `e2e_test.go` の旧 `TestE2E_Estimates_GetByDocumentID`（古い discovery パターン）を削除して M18 版に一本化

## スコープ

### EstimateEntity（11 フィールド）

```go
type EstimateEntity struct {
    ID             int     `json:"id"`
    ClientID       int     `json:"client_id"`
    ProjectID      int     `json:"project_id"`
    Title          string  `json:"title"`
    TotalAmount    float64 `json:"total_amount"`
    Status         string  `json:"status"`
    EstimateDate   string  `json:"estimate_date"`
    ExpirationDate string  `json:"expiration_date"`
    Memo           string  `json:"memo"`
    UpdatedAt      string  `json:"updated_at"`
    CreatedAt      string  `json:"created_at"`
}
```

### GetEstimateRaw 追加

エンドポイント: `GET /v1/documents/estimates/{documentID}`

```go
func (c *Client) GetEstimateRaw(ctx context.Context, documentID int) ([]byte, error)
```

### e2e_test.go クリーンアップ

旧 `TestE2E_Estimates_GetByDocumentID`（`GetProjectWithGroup` typed 経由の古い discovery パターン）を削除し、M18 新テストに一本化。

## 実装内容

### GetEstimateRaw

`internal/boardapi/estimates.go` に追記。`GetVendorRaw`（vendors.go:140）と同パターンで raw HTTP response body を byte-for-byte 返す。通常呼び出し元は `GetEstimate` を使うこと。

### E2E テスト（e2e_estimates_test.go）

- `//go:build e2e` タグ付き
- パッケージ: `boardapi_test`
- `TestE2E_Estimates_Get` 1 本のみ（estimates は document 系のため Get のみ対象）
  1. `findAnyDocumentID(t, client, "estimate")` で projectID と documentID を取得
  2. `requirePositiveID` で両 ID を検証
  3. `GetEstimateRaw` で生レスポンス取得（403/429 は skip せず即 Fatalf）
  4. `dumpJSON` でアーティファクト保存
  5. `testhelper.StrictFieldDiff` で未マップフィールド検出（1 件でも Errorf で失敗）
  6. `json.Unmarshal` で ID 一致確認 + 非 PII ログ（id, title_len, project_id, total）

### e2e_test.go クリーンアップ

- `TestE2E_Estimates_GetByDocumentID` 関数を削除（旧 discovery パターン）
- `--- Estimates (document path) ---` セクションコメントを M18 完了に合わせて更新
- コメントで `e2e_estimates_test.go` への移動を明示

## 検証

```
go build ./...   → OK（エラーなし）
go vet ./...     → OK（警告なし）
go test -count=1 ./... → 全 12 パッケージ PASS
go test -tags e2e -count=1 -run TestE2E_Estimates_Get_CompileOnly ./internal/boardapi/ → [no tests to run]（コンパイル通過）
```

## req 見積

| ステップ | req 数 |
|--------|--------|
| `ListProjectsPage(1, 3)` | 1 |
| `GetProjectWithGroupRaw(projectID, "estimate")` | 最大 3（通常 1） |
| `GetEstimateRaw(documentID)` | 1 |
| **合計** | **~3 req**（見積 5、plan cap 8） |

実消費: 実 API 未実行のため TBD（実行時に本ファイルに追記予定）

## 発見事項

- `GetEstimateRaw` 追加は `GetVendorRaw` と完全同パターン（URL のみ異なる）
- 旧 `TestE2E_Estimates_GetByDocumentID` は `ListProjects`（全ページ走査）+ typed `GetProjectWithGroup` の古いパターンだったが、M17 helper への切り替えで API 消費量を削減
- e2e タグ付きコンパイルは通過（実 API 実行結果は後続実行で追記予定）

## Changelog

| 日付 | 変更 |
|------|------|
| 2026-04-21 | M18 着手・実装完了 |
| 2026-04-21 | `GetEstimateRaw` 追加、`e2e_estimates_test.go` 新規作成、`e2e_test.go` クリーンアップ完了 |
| 2026-04-21 | go build/vet/test 全 Green を確認 |
