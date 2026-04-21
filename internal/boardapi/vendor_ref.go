package boardapi

// VendorRef は VendorBranch/VendorContact 等の子 Entity 内で参照される
// 親 vendor（payee）のサブセット型（{id, name, name_disp, custom_no}）。
// API が nested で返す "vendor" オブジェクトをそのまま保持する。
//
// 注意: 実 API が "vendor" / "payee" どちらのキーで返すかは未確認（データ 0 件のため）。
// ClientRef が "client" で返されることを確認済みなので、vendor 側は "vendor" と推定する。
// データ投入後の smoke テスト（TestE2E_VendorBranches_*）で検証予定。
type VendorRef struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	NameDisp string `json:"name_disp"`
	CustomNo string `json:"custom_no"`
}
