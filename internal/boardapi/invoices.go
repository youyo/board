package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// InvoiceEntity is a BOARD API invoice entity.
// Corresponds to one element in the GET /v1/invoices response.
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

// InvoiceSearchParams is the parameter for SearchInvoices.
type InvoiceSearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListInvoices retrieves all invoices.
// Pagination is automatically handled by ListAll.
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

// GetInvoice retrieves the invoice with the specified ID.
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

// SearchInvoices searches invoices with the given conditions.
// Pagination is automatically handled by ListAll.
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

// ListInvoicesRaw retrieves all invoices and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Each element
// preserves the exact byte content returned by the BOARD API.
//
// Note: the real BOARD API path is /v1/invoices.
//
// Intended for E2E strict field diff; regular callers should use
// ListInvoices.
func (c *Client) ListInvoicesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListInvoicesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetInvoiceRaw retrieves a single invoice and returns the raw HTTP
// response body byte-for-byte.
//
// Note: the real BOARD API path is /v1/invoices/{id}.
//
// Intended for E2E strict field diff; regular callers should use
// GetInvoice.
func (c *Client) GetInvoiceRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/invoices/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchInvoicesRaw retrieves invoices matching the given search parameters and
// returns the raw HTTP response bodies merged across pages as a single JSON
// array. Same byte-preserving guarantee as ListInvoicesRaw.
//
// InvoiceSearchParams exposes 4 filters (ClientID, ProjectID, Status, UpdatedAtFrom).
//
// Intended for E2E strict field diff; regular callers should use
// SearchInvoices.
func (c *Client) SearchInvoicesRaw(ctx context.Context, params InvoiceSearchParams, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchInvoicesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// ListInvoicesPage retrieves a single page of invoices.
func (c *Client) ListInvoicesPage(ctx context.Context, page, perPage int) (*PageResult[InvoiceEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/invoices", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[InvoiceEntity](c, ctx, makeReq, page, perPage)
}
