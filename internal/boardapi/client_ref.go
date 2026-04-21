package boardapi

// ClientRef は ClientBranch/Contact 等の子 Entity 内で参照される
// 親 client のサブセット型（{id, name, name_disp, custom_no}）。
// API が nested で返す "client" オブジェクトをそのまま保持する。
type ClientRef struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	NameDisp string `json:"name_disp"`
	CustomNo string `json:"custom_no"`
}
