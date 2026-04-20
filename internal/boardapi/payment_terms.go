package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PaymentTermEntity is a BOARD API payment term entity.
// Corresponds to one element in the GET /v1/payment_terms response.
type PaymentTermEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PaymentTermSearchParams is the parameter for SearchPaymentTerms.
type PaymentTermSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListPaymentTerms retrieves all payment terms.
// Pagination is automatically handled by ListAll.
func (c *Client) ListPaymentTerms(ctx context.Context) ([]PaymentTermEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
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
	result := make([]PaymentTermEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentTermEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentTerms: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetPaymentTerm retrieves the payment term with the specified ID.
func (c *Client) GetPaymentTerm(ctx context.Context, id int) (*PaymentTermEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payment_terms/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x PaymentTermEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPaymentTerm: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchPaymentTerms searches payment terms with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchPaymentTerms(ctx context.Context, params PaymentTermSearchParams) ([]PaymentTermEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.Name != "" {
			q.Set("name", params.Name)
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
	result := make([]PaymentTermEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentTermEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPaymentTerms: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListPaymentTermsPage retrieves a single page of payment terms.
func (c *Client) ListPaymentTermsPage(ctx context.Context, page, perPage int) (*PageResult[PaymentTermEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[PaymentTermEntity](c, ctx, makeReq, page, perPage)
}

// ListPaymentTermsRaw retrieves all payment terms and returns the raw HTTP
// response bodies merged across pages as a single JSON array.
// Unlike ListPaymentTerms, the returned bytes are byte-preserving: each
// element JSON is exactly what the BOARD API emitted, enabling strict field
// diff in E2E tests to detect keys that are not mapped to PaymentTermEntity.
//
// Intended for E2E strict field diff; regular callers should use ListPaymentTerms.
func (c *Client) ListPaymentTermsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentTermsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetPaymentTermRaw retrieves a single payment term and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetPaymentTerm.
func (c *Client) GetPaymentTermRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payment_terms/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchPaymentTermsRaw retrieves payment terms matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListPaymentTermsRaw.
//
// Intended for E2E strict field diff; regular callers should use SearchPaymentTerms.
func (c *Client) SearchPaymentTermsRaw(ctx context.Context, params PaymentTermSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.Name != "" {
			q.Set("name", params.Name)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPaymentTermsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
