# M42: VendorContactEntity 実 API 準拠への再設計

## 概要

M40 (ContactEntity) と同型のパターンを仕入先担当者側に適用し、
`VendorContactEntity` を実 API レスポンスに準拠した構造に書き換える（Phase J 最終マイルストーン）。

## 前提: データ環境

このアカウントは `/v1/payee_contacts` が 0 件のため、実 API dump が入手不可。
M40 (ContactEntity) の実 API レスポンス構造と BOARD API の一貫性に基づいて推定する。

## 旧 VendorContactEntity（修正前）

```go
type VendorContactEntity struct {
    ID             int    `json:"id"`
    VendorID       int    `json:"vendor_id"`         // 推定幻（API は vendor.id nested と推定）
    VendorBranchID int    `json:"vendor_branch_id"`  // 推定幻（API に存在しないと推定）
    Name           string `json:"name"`              // 推定幻（API は last_name+first_name と推定）
    NameKana       string `json:"name_kana"`         // 推定幻（API に存在しないと推定）
    LastName       string `json:"last_name"`
    FirstName      string `json:"first_name"`
    HonorificTitle string `json:"honorific_title"`
    Title          string `json:"title"`             // 推定幻（API は null 可 *string と推定）
    Department     string `json:"department"`        // 推定幻（API は null 可 *string と推定）
    Email          string `json:"email"`             // 推定幻（API は null 可 *string と推定）
    Phone          string `json:"phone"`             // 推定幻（API に存在しないと推定）
    Note           string `json:"note"`              // 推定幻（API は null 可 *string と推定）
    Memo           string `json:"memo"`              // 推定幻（API に存在しないと推定）
    ArchiveFlg     int    `json:"archive_flg"`
    UpdatedAt      string `json:"updated_at"`
    CreatedAt      string `json:"created_at"`
}
```

id/last_name/first_name/honorific_title/archive_flg/updated_at/created_at の 7 フィールドのみ正しいと推定。
残り 9 フィールドは幻または null 非対応と推定。

## 新 VendorContactEntity（修正後）

### VendorRef 共通型（M41 で作成済み）

```go
type VendorRef struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    NameDisp string `json:"name_disp"`
    CustomNo string `json:"custom_no"`
}
```

### VendorContactEntity 新定義

```go
type VendorContactEntity struct {
    ID             int        `json:"id"`
    Vendor         *VendorRef `json:"vendor"`          // nested 構造（未確認、"vendor" キーと推定）
    LastName       string     `json:"last_name"`
    FirstName      string     `json:"first_name"`
    HonorificTitle string     `json:"honorific_title"`
    Title          *string    `json:"title"`           // null 可
    Department     *string    `json:"department"`      // null 可
    Email          *string    `json:"email"`           // null 可
    Note           *string    `json:"note"`            // null 可
    ArchiveFlg     int        `json:"archive_flg"`
    CreatedAt      string     `json:"created_at"`
    UpdatedAt      string     `json:"updated_at"`
}

func (e VendorContactEntity) VendorID() int {
    if e.Vendor == nil { return 0 }
    return e.Vendor.ID
}

// DisplayName は LastName + FirstName で結合（Name フィールド廃止、ContactEntity と同様）。
func (e VendorContactEntity) DisplayName() string { ... }
```

## 未確認ポイント（Pending Re-verification）

**nested オブジェクトのキー名**: `"vendor"` または `"payee"` のどちらかは未確認。
- ContactEntity が `"client"` キーを使うことが確認済みなので、VendorContact は `"vendor"` と推定。
- データ投入後に `TestE2E_VendorContacts_*` を実行して StrictFieldDiff で検証する。

## 変更ファイル

| ファイル | 変更内容 |
|---|---|
| `internal/boardapi/vendor_contacts.go` | VendorContactEntity 全面再設計 + VendorID() accessor + DisplayName() 更新 |
| `internal/boardapi/vendor_contacts_test.go` | モック JSON を新スキーマに更新（wantKeys 含む） |
| `internal/boardapi/client_test.go` | T124/T126 テストの JSON + Email accessor 修正 |
| `internal/boardapi/e2e_vendor_contacts_test.go` | t.Logf の旧フィールド参照を新フィールド + accessor に修正 |
| `internal/boardapi/display_name_test.go` | TestVendorContactEntity_DisplayName を Name 廃止に合わせて修正 |
| `internal/repository/vendor_contacts.go` | filterVendorContacts: VendorID()/DisplayName()/*string 対応 |
| `internal/repository/vendor_contacts_test.go` | sampleVendorContacts / GetByID test のリテラル修正、Email *string 比較修正 |
| `internal/service/api/service_test.go` | VendorContact 3 テストのリテラル修正 |
| `internal/service/find/find_vendor_test.go` | TestFindVendor_ByID の contacts リテラルを VendorRef nested 構造に変更 |

## 動作確認

- `go build ./...`: Green
- `go vet ./...`: Green
- `go test ./...`: All Green
- E2E smoke: データ 0 件のため Skip（Pending Re-verification）

## Phase J 完走

- M39: ClientBranchEntity 再設計 ✅
- M40: ContactEntity 再設計 ✅
- M41: VendorBranchEntity 再設計 ✅
- M42: VendorContactEntity 再設計 ✅ ← 本 M（Phase J 最終）

Phase J により ClientBranch/Contact/VendorBranch/VendorContact の 4 Entity が実 API 準拠構造に再設計完了。
合計 42 マイルストーン（M01-M38 + Phase J M39-M42）でロードマップ全走完了。
