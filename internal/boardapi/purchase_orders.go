package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PurchaseOrderEntity は BOARD API の発注書エンティティ。
// GET /v1/purchase_orders レスポンスの1要素に対応。
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

// PurchaseOrderSearchParams は SearchPurchaseOrders のパラメータ。
type PurchaseOrderSearchParams struct {
	VendorID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListPurchaseOrders は全発注書を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListPurchaseOrders(ctx context.Context) ([]PurchaseOrderEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/purchase_orders", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]PurchaseOrderEntity, 0, len(items))
	for _, raw := range items {
		var x PurchaseOrderEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseOrders: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetPurchaseOrder は指定 ID の発注書を取得する。
func (c *Client) GetPurchaseOrder(ctx context.Context, id int) (*PurchaseOrderEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/purchase_orders/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x PurchaseOrderEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPurchaseOrder: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchPurchaseOrders は条件付きで発注書を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchPurchaseOrders(ctx context.Context, params PurchaseOrderSearchParams) ([]PurchaseOrderEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/purchase_orders", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
		if params.ProjectID != 0 {
			q.Set("project_id", strconv.Itoa(params.ProjectID))
		}
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]PurchaseOrderEntity, 0, len(items))
	for _, raw := range items {
		var x PurchaseOrderEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPurchaseOrders: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
