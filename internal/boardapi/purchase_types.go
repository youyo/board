package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PurchaseTypeEntity is a BOARD API purchase type entity.
// Corresponds to one element in the GET /v1/purchase_types response.
type PurchaseTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PurchaseTypeSearchParams is the parameter for SearchPurchaseTypes.
type PurchaseTypeSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListPurchaseTypes retrieves all purchase types.
// Pagination is automatically handled by ListAll.
func (c *Client) ListPurchaseTypes(ctx context.Context) ([]PurchaseTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
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
	result := make([]PurchaseTypeEntity, 0, len(items))
	for _, raw := range items {
		var x PurchaseTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetPurchaseType retrieves the purchase type with the specified ID.
func (c *Client) GetPurchaseType(ctx context.Context, id int) (*PurchaseTypeEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x PurchaseTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPurchaseType: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchPurchaseTypes searches purchase types with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchPurchaseTypes(ctx context.Context, params PurchaseTypeSearchParams) ([]PurchaseTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
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
	result := make([]PurchaseTypeEntity, 0, len(items))
	for _, raw := range items {
		var x PurchaseTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPurchaseTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListPurchaseTypesPage retrieves a single page of PurchaseTypeEntity.
func (c *Client) ListPurchaseTypesPage(ctx context.Context, page, perPage int) (*PageResult[PurchaseTypeEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[PurchaseTypeEntity](c, ctx, makeReq, page, perPage)
}

// ListPurchaseTypesRaw retrieves all purchase types and returns the raw HTTP
// response bodies merged across pages as a single JSON array.
// Unlike ListPurchaseTypes, the returned bytes are byte-preserving: each
// element JSON is exactly what the BOARD API emitted, enabling strict field
// diff in E2E tests to detect keys that are not mapped to PurchaseTypeEntity.
//
// Intended for E2E strict field diff; regular callers should use ListPurchaseTypes.
func (c *Client) ListPurchaseTypesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetPurchaseTypeRaw retrieves a single purchase type and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetPurchaseType.
func (c *Client) GetPurchaseTypeRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchPurchaseTypesRaw retrieves purchase types matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListPurchaseTypesRaw.
//
// Intended for E2E strict field diff; regular callers should use SearchPurchaseTypes.
func (c *Client) SearchPurchaseTypesRaw(ctx context.Context, params PurchaseTypeSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPurchaseTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
