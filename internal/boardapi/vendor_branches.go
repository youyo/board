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
