# M41: VendorBranchEntity 実 API 準拠への再設計

## 概要

M39 (ClientBranchEntity) / M40 (ContactEntity) と同型のパターンを仕入先支社側に適用し、
`VendorBranchEntity` を実 API レスポンスに準拠した構造に書き換える（Phase J 第3マイルストーン）。

## 前提: データ環境

このアカウントは `/v1/payee_branches` が 0 件のため、実 API dump が入手不可。
M39 (ClientBranch) の実 API レスポンス構造と BOARD API の一貫性に基づいて推定する。

## 旧 VendorBranchEntity（修正前）

```go
type VendorBranchEntity struct {
    ID         int    `json:"id"`
    VendorID   int    `json:"vendor_id"`        // 推定幻（API は vendor.id nested と推定）
    Name       string `json:"name"`
    PostalCode string `json:"postal_code"`      // 推定幻（API は "zip" と推定）
    Address    string `json:"address"`          // 推定幻（API は address1/address2 と推定）
    Phone      string `json:"phone"`            // 推定幻（API は "tel" と推定）
    Fax        string `json:"fax"`
    Memo       string `json:"memo"`             // 推定幻（API に存在しないと推定）
    UpdatedAt  string `json:"updated_at"`
    CreatedAt  string `json:"created_at"`
}
```

id/name/fax/updated_at/created_at の 5 フィールドのみ正しいと推定。
残り 5 フィールドは幻フィールドと推定。

## 新 VendorBranchEntity（修正後）

### VendorRef 共通型（vendor_ref.go 新規）

```go
type VendorRef struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    NameDisp string `json:"name_disp"`
    CustomNo string `json:"custom_no"`
}
```

ClientRef（M39）と完全に同型。

### VendorBranchEntity 新定義

```go
type VendorBranchEntity struct {
    ID         int        `json:"id"`
    Vendor     *VendorRef `json:"vendor"`      // nested 構造（未確認、"vendor" キーと推定）
    Name       string     `json:"name"`
    Zip        string     `json:"zip"`
    Pref       string     `json:"pref"`
    Address1   string     `json:"address1"`
    Address2   string     `json:"address2"`
    Tel        *string    `json:"tel"`         // null 可
    Fax        *string    `json:"fax"`         // null 可
    ArchiveFlg int        `json:"archive_flg"`
    CreatedAt  string     `json:"created_at"`
    UpdatedAt  string     `json:"updated_at"`
}

func (e VendorBranchEntity) VendorID() int {
    if e.Vendor == nil {
        return 0
    }
    return e.Vendor.ID
}
```

## 未確認ポイント（Pending Re-verification）

**nested オブジェクトのキー名**: `"vendor"` または `"payee"` のどちらかは未確認。
- 実 API パスが `/v1/payees` / `/v1/payee_branches` であることから `"payee"` の可能性もある。
- ClientBranch は `"client"` キーを使うことが確認済みなので、VendorBranch は `"vendor"` と推定。
- データ投入後に `TestE2E_VendorBranches_*` を実行して StrictFieldDiff で検証する。

## 変更ファイル

| ファイル | 変更内容 |
|---|---|
| `internal/boardapi/vendor_ref.go` | 新規作成（VendorRef 共通型） |
| `internal/boardapi/vendor_branches.go` | VendorBranchEntity 全面再設計 + VendorID() accessor |
| `internal/boardapi/vendor_branches_test.go` | モック JSON を新スキーマに更新（wantKeys 含む） |
| `internal/boardapi/client_test.go` | T118/T120 テストの JSON + accessor 修正 |
| `internal/boardapi/e2e_vendor_branches_test.go` | t.Logf の旧フィールド参照を新フィールド + accessor に修正 |
| `internal/repository/vendor_branches.go` | filterVendorBranches: e.VendorID → e.VendorID() |
| `internal/repository/vendor_branches_test.go` | sampleVendorBranches / GetByID test のリテラル修正 |
| `internal/service/api/service_test.go` | VendorBranch 3 テストのリテラル修正 |
| `internal/service/find/find_vendor_test.go` | TestFindVendor_ByID のリテラル修正 |

## 動作確認

- `go build ./...`: Green
- `go vet ./...`: Green
- `go test ./...`: All Green
- E2E smoke: データ 0 件のため Skip（Pending Re-verification）

## Phase J 位置

- M39: ClientBranchEntity 再設計 ✅
- M40: ContactEntity 再設計 ✅
- M41: VendorBranchEntity 再設計 ✅ ← 本 M
- M42: VendorContactEntity 再設計（次）
