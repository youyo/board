# M22: invoices List/Get/Search 厳格突合

## Scope

Phase G（document 系 Raw 層整備）6 件目。
invoices は List/Get/Search の 3 本セット（vendors M16 パターン）。

## リソース情報

| 項目 | 値 |
|------|---|
| Go 名 | invoices |
| 実 API パス | `/v1/invoices` (List/Search), `/v1/invoices/{id}` (Get) |
| Go 型名 | `InvoiceEntity`, `InvoiceSearchParams` |
| Go 型ファイル | `internal/boardapi/invoices.go` |
| フィールド数 | 11 (ID, ClientID, ProjectID, Title, TotalAmount, Status, InvoiceDate, DueDate, Memo, UpdatedAt, CreatedAt) |
| Search パラメータ | ClientID, ProjectID, Status, UpdatedAtFrom |

## 注意事項

- このアカウントでは 11,000 件規模の可能性。List/Search の両方で `boardapi.WithPerPage(1)` を指定してリクエスト数を抑える。
- Search では `UpdatedAtFrom: "2099-01-01"` の far-future フィルタで空結果を狙い、大量ページネーションを回避する。

## API コール数見積

| テスト | リクエスト数 | 備考 |
|--------|------------|------|
| TestE2E_Invoices_List | 1 | WithPerPage(1) で 1 ページのみ |
| TestE2E_Invoices_Get | 2 | List(1件) + Get(1件) |
| TestE2E_Invoices_Search | 1 | WithPerPage(1), UpdatedAtFrom=2099-01-01 で空結果 |
| **合計** | **~4 req** | data 0 items: 3 req |

計画上限: 20 req

## 実施内容

1. `internal/boardapi/invoices.go` に以下を追加:
   - `ListInvoicesRaw(ctx, opts ...ListAllOption) ([]byte, error)` → `/v1/invoices`
   - `GetInvoiceRaw(ctx, id int) ([]byte, error)` → `/v1/invoices/%d`
   - `SearchInvoicesRaw(ctx, params InvoiceSearchParams, opts ...ListAllOption) ([]byte, error)`
2. `internal/boardapi/e2e_invoices_test.go` 新規作成:
   - `TestE2E_Invoices_List`: WithPerPage(1)、StrictFieldDiff、件数ログ
   - `TestE2E_Invoices_Get`: List(1件) → 先頭 ID discovery → Get → StrictFieldDiff、data-dependent skip（0 items）
   - `TestE2E_Invoices_Search`: UpdatedAtFrom=2099-01-01 で空結果、WithPerPage(1)
3. `e2e_test.go` から旧 `TestE2E_Invoices_List`（typed List のみ）を削除
4. ロードマップ更新: M22 ✅、Current Focus → M23

## コミット計画

1. `feat(boardapi): M22 invoices に ListInvoicesRaw / GetInvoiceRaw / SearchInvoicesRaw を追加`
2. `test(e2e): M22 invoices の List/Get/Search E2E を厳格フィールド突合付きで追加`
3. `docs(plans): M22 invoices 完了をロードマップに反映`

## 実績

- 実消費: TBD（実 API 未実行、~4 req 見込み）
- E2E 結果: TBD
