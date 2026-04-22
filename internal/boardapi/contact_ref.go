package boardapi

// ContactRef は Project 等の Entity 内で参照される担当者コンタクトのサブセット型。
// API が nested で返す "contact" オブジェクトを保持する。
// Risk-3: dump では全件 null のため構造未観測。ContactEntity Get 応答からの推定。
// 実データが埋まるプロジェクトが発見された時点で StrictFieldDiff が未マップを検出する。
type ContactRef struct {
	ID        int    `json:"id"`
	LastName  string `json:"last_name"`
	FirstName string `json:"first_name"`
}
