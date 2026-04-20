package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PurchaseOrderEntity is a BOARD API purchase order entity.
// Corresponds to one element in the GET /v1/purchase_orders response.
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

// PurchaseOrderSearchParams is the parameter for SearchPurchaseOrders.
type PurchaseOrderSearchParams struct {
	VendorID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListPurchaseOrders retrieves all purchase orders.
// Pagination is automatically handled by ListAll.
func (c *Client) ListPurchaseOrders(ctx context.Context) ([]PurchaseOrderEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
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

// GetPurchaseOrder retrieves the purchase order with the specified ID.
func (c *Client) GetPurchaseOrder(ctx context.Context, id int) (*PurchaseOrderEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditures/%d", id), nil)
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

// SearchPurchaseOrders searches purchase orders with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchPurchaseOrders(ctx context.Context, params PurchaseOrderSearchParams) ([]PurchaseOrderEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
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

// ListPurchaseOrdersRaw retrieves all purchase orders and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Each element
// preserves the exact byte content returned by the BOARD API.
//
// Note: the real BOARD API path is /v1/expenditures (Go name: purchase_orders).
//
// Intended for E2E strict field diff; regular callers should use
// ListPurchaseOrders.
func (c *Client) ListPurchaseOrdersRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseOrdersRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetPurchaseOrderRaw retrieves a single purchase order and returns the raw HTTP
// response body byte-for-byte.
//
// Note: the real BOARD API path is /v1/expenditures/{id} (Go name: purchase_orders).
//
// Intended for E2E strict field diff; regular callers should use
// GetPurchaseOrder.
func (c *Client) GetPurchaseOrderRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditures/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchPurchaseOrdersRaw retrieves purchase orders matching the given search parameters and
// returns the raw HTTP response bodies merged across pages as a single JSON
// array. Same byte-preserving guarantee as ListPurchaseOrdersRaw.
//
// PurchaseOrderSearchParams exposes 4 filters (VendorID, ProjectID, Status, UpdatedAtFrom).
// Note: the real BOARD API path is /v1/expenditures.
//
// Intended for E2E strict field diff; regular callers should use
// SearchPurchaseOrders.
func (c *Client) SearchPurchaseOrdersRaw(ctx context.Context, params PurchaseOrderSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPurchaseOrdersRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// ListPurchaseOrdersPage retrieves a single page of PurchaseOrderEntity.
func (c *Client) ListPurchaseOrdersPage(ctx context.Context, page, perPage int) (*PageResult[PurchaseOrderEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditures", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[PurchaseOrderEntity](c, ctx, makeReq, page, perPage)
}
