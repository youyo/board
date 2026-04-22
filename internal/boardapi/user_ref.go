package boardapi

// UserRef は Project 等の Entity 内で参照される担当者のサブセット型。
// API が nested で返す "user" オブジェクト {id, last_name, first_name} を保持する。
// dump 観測: {"id":38516996,"last_name":"立花","first_name":"拓也"}
type UserRef struct {
	ID        int    `json:"id"`
	LastName  string `json:"last_name"`
	FirstName string `json:"first_name"`
}
