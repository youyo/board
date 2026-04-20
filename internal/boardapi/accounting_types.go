package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// AccountingTypeEntity is a BOARD API accounting type entity.
// Corresponds to one element in the GET /v1/accounting_types response.
type AccountingTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// AccountingTypeSearchParams is the parameter for SearchAccountingTypes.
type AccountingTypeSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListAccountingTypes retrieves all accounting types.
// Pagination is automatically handled by ListAll.
func (c *Client) ListAccountingTypes(ctx context.Context) ([]AccountingTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
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
	result := make([]AccountingTypeEntity, 0, len(items))
	for _, raw := range items {
		var x AccountingTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListAccountingTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetAccountingType retrieves the accounting type with the specified ID.
func (c *Client) GetAccountingType(ctx context.Context, id int) (*AccountingTypeEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/accounting_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x AccountingTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetAccountingType: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchAccountingTypes searches accounting types with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchAccountingTypes(ctx context.Context, params AccountingTypeSearchParams) ([]AccountingTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
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
	result := make([]AccountingTypeEntity, 0, len(items))
	for _, raw := range items {
		var x AccountingTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchAccountingTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListAccountingTypesPage retrieves a single page of accounting types.
func (c *Client) ListAccountingTypesPage(ctx context.Context, page, perPage int) (*PageResult[AccountingTypeEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[AccountingTypeEntity](c, ctx, makeReq, page, perPage)
}

// ListAccountingTypesRaw retrieves all accounting types and returns the raw
// HTTP response bodies merged across pages as a single JSON array.
// Unlike ListAccountingTypes, the returned bytes are byte-preserving: each
// element JSON is exactly what the BOARD API emitted, enabling strict field
// diff in E2E tests to detect keys that are not mapped to AccountingTypeEntity.
//
// Intended for E2E strict field diff; regular callers should use ListAccountingTypes.
func (c *Client) ListAccountingTypesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListAccountingTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetAccountingTypeRaw retrieves a single accounting type and returns the raw
// HTTP response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetAccountingType.
func (c *Client) GetAccountingTypeRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/accounting_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchAccountingTypesRaw retrieves accounting types matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListAccountingTypesRaw.
//
// Intended for E2E strict field diff; regular callers should use SearchAccountingTypes.
func (c *Client) SearchAccountingTypesRaw(ctx context.Context, params AccountingTypeSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchAccountingTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
