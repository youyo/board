package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PurchaseOrderEntity は BOARD API の発注書エンティティ。
// GET /v1/expenditures および GET /v1/expenditures/{id} のレスポンスに対応する。
// 注意: 実 API パスは /v1/expenditures（Go 名: purchase_orders）。
type PurchaseOrderEntity struct {
	ID           int     `json:"id"`
	VendorID     int     `json:"vendor_id"`
	ProjectID    int     `json:"project_id"`
	Title        string  `json:"title"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
	OrderDate    string  `json:"order_date"`    // ISO 8601 date
	DeliveryDate string  `json:"delivery_date"` // ISO 8601 date
	Memo         string  `json:"memo"`
	UpdatedAt    string  `json:"updated_at"` // ISO 8601
	CreatedAt    string  `json:"created_at"` // ISO 8601
}

// PurchaseOrderListOptions は GET /v1/expenditures のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信されないため、PurchaseOrderListOptions{} は無フィルタのリスト取得となる。
//
// 注意: M54 時点では expenditures の Ransack _eq / _gteq 形式が実 API で有効かは未検証。
// E2E テスト（TestE2E_PurchaseOrders_M54）で確認予定。
// 旧コードでは `vendor_id` を素のクエリパラメータとして使用していた。
//
// Introduced in M54 as the Phase L purchase_orders migration target.
// Replaces the pre-M54 PurchaseOrderSearchParams struct.
type PurchaseOrderListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）。
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// purchase_orders 専用
	VendorIDEq    int    // 取引先 ID 完全一致
	ProjectIDEq   int    // プロジェクト ID 完全一致
	StatusEq      string // ステータス完全一致
	ResponseGroup string // "small" / "large"
}

// PurchaseOrderSearchParams は SearchPurchaseOrders の後方互換のために残す。
//
// Deprecated: M54 以降は PurchaseOrderListOptions を使用すること。
// M57 で一括削除予定。
type PurchaseOrderSearchParams struct {
	VendorID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// buildPurchaseOrdersQuery は PurchaseOrderListOptions を Ransack スタイルのクエリ文字列に変換する。
func buildPurchaseOrdersQuery(opts PurchaseOrderListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("vendor_id", opts.VendorIDEq).
		IntEq("project_id", opts.ProjectIDEq).
		StrEq("status", opts.StatusEq).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		ResponseGroup(opts.ResponseGroup).
		Encode()
}

// ListPurchaseOrders は PurchaseOrderListOptions でフィルタした発注書一覧を返す。
// ページネーションは ListAllWithResult が内部で処理する。
//
// 注意: 実 API パスは /v1/expenditures（Go 名: purchase_orders）。
//
// Pass PurchaseOrderListOptions{} for an unfiltered list of all purchase orders.
func (c *Client) ListPurchaseOrders(ctx context.Context, opts PurchaseOrderListOptions) (*ListResult[PurchaseOrderEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPurchaseOrdersQuery(opts, page, pp)
		return req, nil
	}
	var listOpts []ListAllOption
	if perPage > 0 {
		listOpts = append(listOpts, WithPerPage(perPage))
	}
	raw, err := c.ListAllWithResult(ctx, makeReq, listOpts...)
	if err != nil {
		return nil, err
	}
	items := make([]PurchaseOrderEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x PurchaseOrderEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseOrders: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[PurchaseOrderEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetPurchaseOrder は指定 ID の発注書を返す。
// 注意: 実 API パスは /v1/expenditures/{id}。
func (c *Client) GetPurchaseOrder(ctx context.Context, id int) (*ItemResult[PurchaseOrderEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditures/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x PurchaseOrderEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPurchaseOrder: unmarshal: " + err.Error()}
	}
	return &ItemResult[PurchaseOrderEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListPurchaseOrdersRaw は発注書一覧を生 JSON バイト列と最終ページのヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は ListPurchaseOrders を使うこと。
// 注意: 実 API パスは /v1/expenditures。
func (c *Client) ListPurchaseOrdersRaw(ctx context.Context, opts PurchaseOrderListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPurchaseOrdersQuery(opts, page, pp)
		return req, nil
	}
	var listOpts []ListAllOption
	if perPage > 0 {
		listOpts = append(listOpts, WithPerPage(perPage))
	}
	raw, err := c.ListAllWithResult(ctx, makeReq, listOpts...)
	if err != nil {
		return nil, nil, err
	}
	out, err := json.Marshal(raw.Items)
	if err != nil {
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseOrdersRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetPurchaseOrderRaw は指定 ID の発注書を生 JSON バイト列とヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は GetPurchaseOrder を使うこと。
// 注意: 実 API パスは /v1/expenditures/{id}。
func (c *Client) GetPurchaseOrderRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditures/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
