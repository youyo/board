package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// VendorEntity is a BOARD API vendor entity.
// Corresponds to one element in the GET /v1/vendors response.
type VendorEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// VendorSearchParams is the parameter for SearchVendors.
type VendorSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListVendors retrieves all vendors.
// Pagination is automatically handled by ListAll.
func (c *Client) ListVendors(ctx context.Context) ([]VendorEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
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
	result := make([]VendorEntity, 0, len(items))
	for _, raw := range items {
		var x VendorEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendors: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetVendor retrieves the vendor with the specified ID.
func (c *Client) GetVendor(ctx context.Context, id int) (*VendorEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payees/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x VendorEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendor: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchVendors searches vendors with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchVendors(ctx context.Context, params VendorSearchParams) ([]VendorEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
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
	result := make([]VendorEntity, 0, len(items))
	for _, raw := range items {
		var x VendorEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendors: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListVendorsRaw retrieves all vendors and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Each element
// preserves the exact byte content returned by the BOARD API.
//
// Note: the real BOARD API path is /v1/payees.
//
// Intended for E2E strict field diff; regular callers should use
// ListVendors.
func (c *Client) ListVendorsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetVendorRaw retrieves a single vendor and returns the raw HTTP
// response body byte-for-byte.
//
// Note: the real BOARD API path is /v1/payees/{id}.
//
// Intended for E2E strict field diff; regular callers should use
// GetVendor.
func (c *Client) GetVendorRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payees/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchVendorsRaw retrieves vendors matching the given search parameters and
// returns the raw HTTP response bodies merged across pages as a single JSON
// array. Same byte-preserving guarantee as ListVendorsRaw.
//
// VendorSearchParams exposes 2 filters (Name, UpdatedAtFrom).
// Note that the BOARD API has been observed to ignore the `name` filter across
// 9 consecutive milestones (M03-M13), so the Name value in Search only
// exercises request encoding, not server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use
// SearchVendors.
func (c *Client) SearchVendorsRaw(ctx context.Context, params VendorSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// ListVendorsPage retrieves a single page of VendorEntity.
func (c *Client) ListVendorsPage(ctx context.Context, page, perPage int) (*PageResult[VendorEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[VendorEntity](c, ctx, makeReq, page, perPage)
}
