package boardapi

// HubspotRef は Project 等の Entity 内で参照される HubSpot 連携情報のサブセット型。
// API が nested で返す "hubspot" オブジェクトを保持する。
// dump 観測: {"hubspot_deal_id":null}
type HubspotRef struct {
	HubspotDealID *string `json:"hubspot_deal_id"`
}
