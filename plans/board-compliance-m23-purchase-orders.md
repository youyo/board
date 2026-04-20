# M23: purchase_orders List/Get/Search 厳格突合

## Scope

Phase G（vendor-side Raw 層整備）7 件目。
purchase_orders は List/Get/Search の 3 本セット（vendors M16 パターン）。

## リソース情報

| 項目 | 値 |
|------|---|
| Go 名 | purchase_orders |
| 実 API パス | `/v1/expenditures` (List/Search), `/v1/expenditures/{id}` (Get) |
| Go 型名 | `PurchaseOrderEntity`, `PurchaseOrderSearchParams` |
| Go 型ファイル | `internal/boardapi/purchase_orders.go` |
| フィールド数 | 11 (ID, VendorID, ProjectID, Title, TotalAmount, Status, OrderDate, DeliveryDate, Memo, UpdatedAt, CreatedAt) |
| Search パラメータ | VendorID, ProjectID, Status, UpdatedAtFrom |

## 注意事項

- Go 名（purchase_orders）と実 API パス（/v1/expenditures）が異なる（vendors と同じ命名不一致パターン）。
- 既存の旧 `TestE2E_PurchaseOrders_List`（typed List のみ）を e2e_test.go から削除し厳格突合版に一本化。
- 旧 `TestE2E_Payments_List` も M24 相当として同時に e2e_test.go から削除（対称性）。

## API コール数見積

| テスト | リクエスト数 | 備考 |
|--------|------------|------|
| TestE2E_PurchaseOrders_List | 1 | 全件取得（データ量は不明） |
| TestE2E_PurchaseOrders_Get | 2 | List(1件) + Get(1件) |
| TestE2E_PurchaseOrders_Search | 1 | WithPerPage(1), UpdatedAtFrom=2099-01-01 で空結果 |
| **合計** | **~4 req** | data 0 items: 3 req |

計画上限: 10 req

## 実施内容

1. `internal/boardapi/purchase_orders.go` に以下を追加:
   - `ListPurchaseOrdersRaw(ctx, opts ...ListAllOption) ([]byte, error)` → `/v1/expenditures`
   - `GetPurchaseOrderRaw(ctx, id int) ([]byte, error)` → `/v1/expenditures/%d`
   - `SearchPurchaseOrdersRaw(ctx, params PurchaseOrderSearchParams, opts ...ListAllOption) ([]byte, error)`
2. `internal/boardapi/e2e_purchase_orders_test.go` 新規作成:
   - `TestE2E_PurchaseOrders_List`: StrictFieldDiff、件数ログ
   - `TestE2E_PurchaseOrders_Get`: List(1件) → 先頭 ID discovery → Get → StrictFieldDiff、data-dependent skip（0 items）
   - `TestE2E_PurchaseOrders_Search`: UpdatedAtFrom=2099-01-01 で空結果、WithPerPage(1)
3. `e2e_test.go` から旧 `TestE2E_PurchaseOrders_List` と `TestE2E_Payments_List` を削除（対称的削除）
4. ロードマップ更新: M23 ✅、Current Focus → M24

## コミット計画

1. `feat(boardapi): M23 purchase_orders に ListPurchaseOrdersRaw / GetPurchaseOrderRaw / SearchPurchaseOrdersRaw を追加`
2. `test(e2e): M23 purchase_orders の List/Get/Search E2E を厳格フィールド突合付きで追加`
3. `docs(plans): M23 purchase_orders 完了をロードマップに反映`

## 実績

- 実消費: TBD（実 API 未実行、~4 req 見込み）
- E2E 結果: TBD
