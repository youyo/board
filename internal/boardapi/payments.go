package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PaymentEntity is a BOARD API payment entity.
// Corresponds to one element in the GET /v1/payments response.
type PaymentEntity struct {
	ID              int     `json:"id"`
	VendorID        int     `json:"vendor_id"`
	PurchaseOrderID int     `json:"purchase_order_id"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status"`
	PaymentDate     string  `json:"payment_date"` // ISO 8601 date
	Memo            string  `json:"memo"`
	UpdatedAt       string  `json:"updated_at"` // ISO 8601
	CreatedAt       string  `json:"created_at"` // ISO 8601
}

// PaymentSearchParams is the parameter for SearchPayments.
type PaymentSearchParams struct {
	VendorID        int
	PurchaseOrderID int
	Status          string
	UpdatedAtFrom   string
}

// ListPayments retrieves all payments.
// Pagination is automatically handled by ListAll.
func (c *Client) ListPayments(ctx context.Context) ([]PaymentEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_payments", nil)
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
	result := make([]PaymentEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPayments: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetPayment retrieves the payment with the specified ID.
func (c *Client) GetPayment(ctx context.Context, id int) (*PaymentEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_payments/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x PaymentEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPayment: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchPayments searches payments with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchPayments(ctx context.Context, params PaymentSearchParams) ([]PaymentEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_payments", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
		if params.PurchaseOrderID != 0 {
			q.Set("purchase_order_id", strconv.Itoa(params.PurchaseOrderID))
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
	result := make([]PaymentEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPayments: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListPaymentsPage retrieves a single page of PaymentEntity.
func (c *Client) ListPaymentsPage(ctx context.Context, page, perPage int) (*PageResult[PaymentEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_payments", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[PaymentEntity](c, ctx, makeReq, page, perPage)
}
