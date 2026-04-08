package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ReceiptEntity is a BOARD API receipt entity.
// Corresponds to one element in the GET /v1/receipts response.
type ReceiptEntity struct {
	ID          int     `json:"id"`
	ClientID    int     `json:"client_id"`
	ProjectID   int     `json:"project_id"`
	Title       string  `json:"title"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	ReceiptDate string  `json:"receipt_date"` // ISO 8601 date
	Memo        string  `json:"memo"`
	UpdatedAt   string  `json:"updated_at"` // ISO 8601
	CreatedAt   string  `json:"created_at"` // ISO 8601
}

// ReceiptSearchParams is the parameter for SearchReceipts.
type ReceiptSearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListReceipts retrieves all receipts.
// Pagination is automatically handled by ListAll.
func (c *Client) ListReceipts(ctx context.Context) ([]ReceiptEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/receipts", nil)
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
	result := make([]ReceiptEntity, 0, len(items))
	for _, raw := range items {
		var x ReceiptEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListReceipts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetReceipt retrieves the receipt with the specified ID.
func (c *Client) GetReceipt(ctx context.Context, id int) (*ReceiptEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/receipts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ReceiptEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetReceipt: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchReceipts searches receipts with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchReceipts(ctx context.Context, params ReceiptSearchParams) ([]ReceiptEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/receipts", nil)
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
	result := make([]ReceiptEntity, 0, len(items))
	for _, raw := range items {
		var x ReceiptEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchReceipts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
