# M30: FindVendor / FindPurchaseOrder / FindPayment 新規 E2E（Phase H 6 件目）

## Scope

Phase H（service/find 層）6 件目。
vendor 側 3 Find メソッドの新規 E2E を追加する。

## 対象コード

| ファイル | 役割 |
|---------|------|
| `internal/service/find/find_vendor.go` (84 行) | FindVendor: ID/Name/Text モード、VendorResult{Vendor, Branches[], Contacts[]} |
| `internal/service/find/find_purchase_order.go` (112 行) | FindPurchaseOrder: ID/VendorName/ProjectName/Text/Status モード、PurchaseOrderResult{PurchaseOrder, Vendor*, Project*} |
| `internal/service/find/find_payment.go` (111 行) | FindPayment: ID/VendorName/PurchaseOrderID/Text/Status モード、PaymentResult{Payment, Vendor*} |
| `internal/service/find/e2e_test.go` | E2E テスト追加対象 |

## API コール数

| テスト | リクエスト数 | 備考 |
|--------|------------|------|
| TestE2E_FindVendor_StrictEnrichment | ~1 (SKIP) | ListVendorsPage 1 → 0 件 → SKIP |
| TestE2E_FindVendor_ByName / ByText | 0 (static SKIP) | vendors 0 件 |
| TestE2E_FindPurchaseOrder_ByID_Strict | ~1 (SKIP) | ListPurchaseOrdersPage 1 → 0 件 → SKIP |
| TestE2E_FindPurchaseOrder_ByVendorName_Strict | 0 (static SKIP) | vendors 0 件 |
| TestE2E_FindPurchaseOrder_ByProjectName_Strict | 0 (static SKIP) | cache-warm SKIP |
| TestE2E_FindPurchaseOrder_ByText_Strict | ~1 (SKIP) | ListPurchaseOrdersPage 1 → 0 件 → SKIP |
| TestE2E_FindPurchaseOrder_ByStatus_Strict | ~1 (SKIP) | ListPurchaseOrdersPage 1 → 0 件 → SKIP |
| TestE2E_FindPayment_ByID_Strict | ~1 (SKIP) | ListPaymentsPage 1 → 0 件 → SKIP |
| TestE2E_FindPayment_ByVendorName_Strict | 0 (static SKIP) | vendors 0 件 |
| TestE2E_FindPayment_ByPurchaseOrderID_Strict | 0 (static SKIP) | purchase_orders 0 件 |
| TestE2E_FindPayment_ByText_Strict | ~1 (SKIP) | ListPaymentsPage 1 → 0 件 → SKIP |
| TestE2E_FindPayment_ByStatus_Strict | ~1 (SKIP) | ListPaymentsPage 1 → 0 件 → SKIP |
| **合計** | **~7 req** | 全て SKIP の確認のみ |

## Entity 構造

### VendorEntity (vendors.go)

```go
type VendorEntity struct {
    ID        int    `json:"id"`
    Name      string `json:"name"`
    Code      string `json:"code"`
    Memo      string `json:"memo"`
    UpdatedAt string `json:"updated_at"`
    CreatedAt string `json:"created_at"`
}
```

### VendorBranchEntity (vendor_branches.go)

```go
type VendorBranchEntity struct {
    ID         int    `json:"id"`
    VendorID   int    `json:"vendor_id"`
    Name       string `json:"name"`
    PostalCode string `json:"postal_code"`
    Address    string `json:"address"`
    Phone      string `json:"phone"`
    Fax        string `json:"fax"`
    Memo       string `json:"memo"`
    UpdatedAt  string `json:"updated_at"`
    CreatedAt  string `json:"created_at"`
}
```

### VendorContactEntity (vendor_contacts.go)

```go
type VendorContactEntity struct {
    ID             int    `json:"id"`
    VendorID       int    `json:"vendor_id"`
    VendorBranchID int    `json:"vendor_branch_id"`
    Name           string `json:"name"`
    // ... (17 fields total)
}
```

### PurchaseOrderEntity (purchase_orders.go)

```go
type PurchaseOrderEntity struct {
    ID           int     `json:"id"`
    VendorID     int     `json:"vendor_id"`
    ProjectID    int     `json:"project_id"`
    Title        string  `json:"title"`
    TotalAmount  float64 `json:"total_amount"`
    Status       string  `json:"status"`
    OrderDate    string  `json:"order_date"`
    DeliveryDate string  `json:"delivery_date"`
    Memo         string  `json:"memo"`
    UpdatedAt    string  `json:"updated_at"`
    CreatedAt    string  `json:"created_at"`
}
```

注意: `Total` フィールドは存在しない（`TotalAmount float64`）。M27 OrderEntity.Total (string) と異なる。

### PaymentEntity (payments.go)

```go
type PaymentEntity struct {
    ID              int     `json:"id"`
    VendorID        int     `json:"vendor_id"`
    PurchaseOrderID int     `json:"purchase_order_id"`
    Amount          float64 `json:"amount"`
    Status          string  `json:"status"`
    PaymentDate     string  `json:"payment_date"`
    Memo            string  `json:"memo"`
    UpdatedAt       string  `json:"updated_at"`
    CreatedAt       string  `json:"created_at"`
}
```

## 実施内容

### データ環境確認

実施前に API で確認:
- `ListVendorsPage(1, 1)`: 0 件
- `ListPurchaseOrdersPage(1, 1)`: 0 件
- `ListPaymentsPage(1, 1)`: 0 件

→ 当該アカウントは vendor 系データ全 0 件。全テストが SKIP。

### TestE2E_FindVendor_StrictEnrichment 設計

データがある場合:
1. `ListVendorsPage(1, 1)` で vendor を 1 件取得
2. `FindVendor(ID)` を呼び出し VendorResult を取得
3. `SearchVendorBranchesRaw(VendorBranchSearchParams{VendorID})` と件数・ID 集合を突合
4. `SearchVendorContactsRaw(VendorContactSearchParams{VendorID})` と件数・ID 集合を突合

**M25 同型バグ検出の可能性**:
`VendorBranchRepository.Search` と `VendorContactRepository.Search` は in-memory filter パターンを使用している。
VendorID フィルタ実装:
```go
func filterVendorBranches(entities []boardapi.VendorBranchEntity, params boardapi.VendorBranchSearchParams) []boardapi.VendorBranchEntity {
    for _, e := range entities {
        if params.VendorID != 0 && e.VendorID != params.VendorID {
            continue
        }
        // ...
    }
}
```

VendorBranchEntity.VendorID は `json:"vendor_id"` フラットマッピングのため、
BOARD API がフラット `vendor_id` フィールドを返すなら in-memory filter は正常動作する。
BOARD API がネスト構造（M09 の `client:{id,name}` のような `vendor:{id,...}`）を返すなら
VendorID が常に 0 になり全件除外される（M25 と同じバグ）。

データ投入後に StrictEnrichment テストで判定される。

### FindPurchaseOrder / FindPayment の注意点

- `ByVendorName`: vendors 0 件 → static SKIP
- `ByProjectName`: cascade-fetch リスク（全 project → 全件 PO fetch）→ cache-warm SKIP（M27/M28/M29 と同パターン）
- `ByPurchaseOrderID` (Payment): purchase_orders 0 件 → static SKIP

## コミット

1. `test(e2e): M30 FindVendor/FindPurchaseOrder/FindPayment の厳格検証 E2E を追加`
2. `docs(plans): M30 vendor 系 Find 完了をロードマップに反映`

## 実績

- 変更ファイル: `internal/service/find/e2e_test.go` + `plans/board-compliance-m30-find-vendor-side.md` + `plans/board-compliance-roadmap.md`
- E2E 結果: 全 13 テスト SKIP（PASS 相当）、`go test -tags e2e` PASS
- build/vet/test: 全 Green（全 12 パッケージ）
- 実消費: ~7 req（SKIP 確認のみ）
- 発見事項:
  - VendorBranchRepository/VendorContactRepository に M25 同型 enrichment バグ潜在（データ 0 件のため未表面化）
  - 全 vendor 系データ（vendors/vendor_branches/vendor_contacts/purchase_orders/payments）が 0 件
- Pending Re-verification 追加: 3 件（M30 vendors/purchase_orders/payments find テスト）
