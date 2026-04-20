# M24: payments List/Get/Search 厳格突合

## Scope

Phase G（vendor-side Raw 層整備）8 件目・**Phase G 完走**。
payments は List/Get/Search の 3 本セット（vendors M16 パターン）。

## リソース情報

| 項目 | 値 |
|------|---|
| Go 名 | payments |
| 実 API パス | `/v1/expenditure_payments` (List/Search), `/v1/expenditure_payments/{id}` (Get) |
| Go 型名 | `PaymentEntity`, `PaymentSearchParams` |
| Go 型ファイル | `internal/boardapi/payments.go` |
| フィールド数 | 9 (ID, VendorID, PurchaseOrderID, Amount, Status, PaymentDate, Memo, UpdatedAt, CreatedAt) |
| Search パラメータ | VendorID, PurchaseOrderID, Status, UpdatedAtFrom |

## 注意事項

- Go 名（payments）と実 API パス（/v1/expenditure_payments）が異なる（purchase_orders の /v1/expenditures と同じ命名不一致パターン）。
- e2e_test.go に payments 関連の古いテストは既に M23 で削除済み（TestE2E_Payments_List を対称的削除）。

## API コール数見積

| テスト | リクエスト数 | 備考 |
|--------|------------|------|
| TestE2E_Payments_List | 1 | 全件取得 |
| TestE2E_Payments_Get | 2 | List(1件) + Get(1件) |
| TestE2E_Payments_Search | 1 | WithPerPage(1), UpdatedAtFrom=2099-01-01 で空結果 |
| **合計** | **~4 req** | data 0 items: 3 req |

計画上限: 10 req

## 実施内容

1. `internal/boardapi/payments.go` に以下を追加:
   - `ListPaymentsRaw(ctx, opts ...ListAllOption) ([]byte, error)` → `/v1/expenditure_payments`
   - `GetPaymentRaw(ctx, id int) ([]byte, error)` → `/v1/expenditure_payments/%d`
   - `SearchPaymentsRaw(ctx, params PaymentSearchParams, opts ...ListAllOption) ([]byte, error)`
2. `internal/boardapi/e2e_payments_test.go` 新規作成（Phase G 8件目・Phase G 完走）:
   - `TestE2E_Payments_List`: StrictFieldDiff、件数ログ
   - `TestE2E_Payments_Get`: List(1件) → 先頭 ID discovery → Get → StrictFieldDiff、data-dependent skip（0 items）
   - `TestE2E_Payments_Search`: UpdatedAtFrom=2099-01-01 で空結果、WithPerPage(1)
3. ロードマップ更新: M24 ✅、Current Focus → M25 FindClient 厳格化、Phase G 完走を反映

## コミット計画

1. `feat(boardapi): M24 payments に ListPaymentsRaw / GetPaymentRaw / SearchPaymentsRaw を追加`
2. `test(e2e): M24 payments の List/Get/Search E2E を厳格フィールド突合付きで追加（Phase G 完走）`
3. `docs(plans): M24 payments 完了・Phase G 完走をロードマップに反映`

## 実績

- 実消費: TBD（実 API 未実行、~4 req 見込み）
- E2E 結果: TBD
