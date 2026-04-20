# M21: receipts Get 厳格突合（Phase G 5 件目）

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | M21 / Phase G 5 件目 |
| ロードマップ | plans/board-compliance-roadmap.md |
| 対象 | `internal/boardapi/receipts.go`（GetReceiptRaw 追加）、`internal/boardapi/e2e_receipts_test.go`（新規） |
| スコープ | `ReceiptEntity`（10 フィールド）の厳格フィールド突合。`GetReceiptRaw` を追加し、M17 で確立した `findAnyDocumentID` helper 経由で documentID を discovery し E2E Get テストを実施。 |
| 見積 | ~10 req（ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetReceiptRaw 1 = 3 req 期待）|
| 前提 | M17 `findAnyDocumentID(t, client, docType)` helper が `e2e_helpers_test.go` に実装済み |

## 背景

M20 で deliveries の GetDeliveryRaw + e2e_deliveries_test.go を確立し（Phase G 4 件目）、Phase G の document 系 E2E テスト基盤が整った。M21 では receipts を 5 件目の適用対象として:

1. `GetReceiptRaw` を `receipts.go` に追加（deliveries.go の `GetDeliveryRaw` と同パターン）
2. `e2e_receipts_test.go` を新規作成し M17 helper 経由の厳格突合 E2E テストを追加

## スコープ

### ReceiptEntity（10 フィールド）

```go
type ReceiptEntity struct {
    ID          int     `json:"id"`
    ClientID    int     `json:"client_id"`
    ProjectID   int     `json:"project_id"`
    Title       string  `json:"title"`
    TotalAmount float64 `json:"total_amount"`
    Status      string  `json:"status"`
    ReceiptDate string  `json:"receipt_date"` // ISO 8601 date
    Memo        string  `json:"memo"`
    UpdatedAt   string  `json:"updated_at"`   // ISO 8601
    CreatedAt   string  `json:"created_at"`   // ISO 8601
}
```

### GetReceiptRaw 追加

エンドポイント: `GET /v1/documents/receipts/{documentID}`

```go
func (c *Client) GetReceiptRaw(ctx context.Context, documentID int) ([]byte, error)
```

## 実装内容

### GetReceiptRaw

`internal/boardapi/receipts.go` に追記。`GetDeliveryRaw`（deliveries.go）と同パターンで raw HTTP response body を byte-for-byte 返す。通常呼び出し元は `GetReceipt` を使うこと。

### E2E テスト（e2e_receipts_test.go）

- `//go:build e2e` タグ付き
- パッケージ: `boardapi_test`
- `TestE2E_Receipts_Get` 1 本のみ（receipts は document 系のため Get のみ対象）
  1. `findAnyDocumentID(t, client, "receipt")` で projectID と documentID を取得
  2. `requirePositiveID` で両 ID を検証
  3. `GetReceiptRaw` で生レスポンス取得（403/429 は skip せず即 Fatalf）
  4. `dumpJSON` でアーティファクト保存
  5. `testhelper.StrictFieldDiff` で未マップフィールド検出（1 件でも Errorf で失敗）
  6. `json.Unmarshal` で ID/ProjectID 一致確認 + 非 PII ログ（id, title_len, project_id, total, receipt_date）

## 検証

```
go build ./...   → OK（エラーなし）
go vet ./...     → OK（警告なし）
go test -count=1 ./... → 全パッケージ PASS
go test -tags e2e -count=1 -run TestE2E_Receipts_Get_CompileOnly ./internal/boardapi/ → [no tests to run]（コンパイル通過）
```

## req 見積

| ステップ | req 数 |
|--------|--------|
| `ListProjectsPage(1, 3)` | 1 |
| `GetProjectWithGroupRaw(projectID, "receipt")` | 最大 3（通常 1） |
| `GetReceiptRaw(documentID)` | 1 |
| **合計** | **~3 req**（見積 10、plan cap 15） |

実消費: 実 API 未実行のため TBD（実行時に本ファイルに追記予定）

## 発見事項

- `GetReceiptRaw` 追加は `GetDeliveryRaw`（M20）と完全同パターン（URL のみ異なる）
- `e2e_test.go` のクリーンアップは不要（M20 時点で receipts 関連の古いテストは存在しなかった）
- e2e タグ付きコンパイルは通過（実 API 実行結果は後続実行で追記予定）
- ログに `receipt_date` を追加（deliveries の `delivery_date` に相当、PII ではない）

## Changelog

| 日付 | 変更 |
|------|------|
| 2026-04-21 | M21 着手・実装完了 |
| 2026-04-21 | `GetReceiptRaw` 追加、`e2e_receipts_test.go` 新規作成 |
| 2026-04-21 | go build/vet/test 全 Green を確認 |
