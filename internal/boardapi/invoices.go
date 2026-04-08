package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// InvoiceEntity は BOARD API の請求エンティティ。
// GET /v1/invoices レスポンスの1要素に対応。
type InvoiceEntity struct {
	ID          int     `json:"id"`
	ClientID    int     `json:"client_id"`
	ProjectID   int     `json:"project_id"`
	Title       string  `json:"title"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	InvoiceDate string  `json:"invoice_date"` // ISO 8601 date
	DueDate     string  `json:"due_date"`     // ISO 8601 date
	Memo        string  `json:"memo"`
	UpdatedAt   string  `json:"updated_at"` // ISO 8601
	CreatedAt   string  `json:"created_at"` // ISO 8601
}

// InvoiceSearchParams は SearchInvoices のパラメータ。
type InvoiceSearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListInvoices は全請求を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListInvoices(ctx context.Context) ([]InvoiceEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/invoices", nil)
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
	result := make([]InvoiceEntity, 0, len(items))
	for _, raw := range items {
		var x InvoiceEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListInvoices: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetInvoice は指定 ID の請求を取得する。
func (c *Client) GetInvoice(ctx context.Context, id int) (*InvoiceEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/invoices/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x InvoiceEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetInvoice: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchInvoices は条件付きで請求を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchInvoices(ctx context.Context, params InvoiceSearchParams) ([]InvoiceEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/invoices", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
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
	result := make([]InvoiceEntity, 0, len(items))
	for _, raw := range items {
		var x InvoiceEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchInvoices: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
