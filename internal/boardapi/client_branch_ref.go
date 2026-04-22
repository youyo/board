package boardapi

// ClientBranchRef は Project 等の Entity 内で参照される取引先支店のサブセット型。
// API が nested で返す "client_branch" オブジェクトを保持する。
// Risk-3: dump では null 観測のみ、実構造未確認。ClientBranchEntity から推定。
// 実データが埋まるプロジェクトが発見された時点で StrictFieldDiff が未マップを検出する。
type ClientBranchRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
