package boardapi

// CompanyBranchRef は Project 等の Entity 内で参照される自社支店のサブセット型。
// API が nested で返す "company_branch" オブジェクトを保持する。
// Risk-3: dump では null 観測のみ、実構造未確認。
// 実データが埋まるプロジェクトが発見された時点で StrictFieldDiff が未マップを検出する。
type CompanyBranchRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
