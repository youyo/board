package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// VendorBranchEntity is a BOARD API vendor branch entity.
// Corresponds to one element in the GET /v1/vendor_branches response.
type VendorBranchEntity struct {
	ID         int    `json:"id"`
	VendorID   int    `json:"vendor_id"`
	Name       string `json:"name"`
	PostalCode string `json:"postal_code"`
	Address    string `json:"address"`
	Phone      string `json:"phone"`
	Fax        string `json:"fax"`
	Memo       string `json:"memo"`
	UpdatedAt  string `json:"updated_at"` // ISO 8601
	CreatedAt  string `json:"created_at"` // ISO 8601
}

// VendorBranchSearchParams is the parameter for SearchVendorBranches.
type VendorBranchSearchParams struct {
	VendorID      int
	Name          string
	UpdatedAtFrom string
}

// ListVendorBranches retrieves all vendor branches.
// Pagination is automatically handled by ListAll.
func (c *Client) ListVendorBranches(ctx context.Context) ([]VendorBranchEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
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
	result := make([]VendorBranchEntity, 0, len(items))
	for _, raw := range items {
		var x VendorBranchEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorBranches: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetVendorBranch retrieves the vendor branch with the specified ID.
func (c *Client) GetVendorBranch(ctx context.Context, id int) (*VendorBranchEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x VendorBranchEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendorBranch: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchVendorBranches searches vendor branches with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchVendorBranches(ctx context.Context, params VendorBranchSearchParams) ([]VendorBranchEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
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
	result := make([]VendorBranchEntity, 0, len(items))
	for _, raw := range items {
		var x VendorBranchEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorBranches: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListVendorBranchesRaw retrieves all vendor branches and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Unlike
// ListVendorBranches, the returned bytes are byte-preserving: each element
// JSON is exactly what the BOARD API emitted, enabling strict field diff in
// E2E tests to detect keys that are not mapped to VendorBranchEntity.
//
// Note: the real BOARD API path is /v1/payee_branches (not /v1/vendor_branches).
//
// Intended for E2E strict field diff; regular callers should use
// ListVendorBranches.
func (c *Client) ListVendorBranchesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetVendorBranchRaw retrieves a single vendor branch and returns the raw HTTP
// response body byte-for-byte.
//
// Note: the real BOARD API path is /v1/payee_branches/{id}.
//
// Intended for E2E strict field diff; regular callers should use
// GetVendorBranch.
func (c *Client) GetVendorBranchRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchVendorBranchesRaw retrieves vendor branches matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListVendorBranchesRaw.
//
// VendorBranchSearchParams exposes 3 filters (VendorID, Name, UpdatedAtFrom).
// Note that the BOARD API has been observed to ignore the `name` filter across
// 9 consecutive milestones (M03-M13), so the Name value in Search only
// exercises request encoding, not server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use
// SearchVendorBranches.
func (c *Client) SearchVendorBranchesRaw(ctx context.Context, params VendorBranchSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// ListVendorBranchesPage retrieves a single page of VendorBranchEntity.
func (c *Client) ListVendorBranchesPage(ctx context.Context, page, perPage int) (*PageResult[VendorBranchEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[VendorBranchEntity](c, ctx, makeReq, page, perPage)
}
