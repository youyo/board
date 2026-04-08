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
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/vendors", nil)
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
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/vendors/%d", id), nil)
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
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/vendors", nil)
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
