package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectTypeEntity is a BOARD API project type entity.
// Corresponds to one element in the GET /v1/project_types response.
type ProjectTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// ProjectTypeSearchParams is the parameter for SearchProjectTypes.
type ProjectTypeSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListProjectTypes retrieves all project types.
// Pagination is automatically handled by ListAll.
func (c *Client) ListProjectTypes(ctx context.Context) ([]ProjectTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
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
	result := make([]ProjectTypeEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetProjectType retrieves the project type with the specified ID.
func (c *Client) GetProjectType(ctx context.Context, id int) (*ProjectTypeEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectType: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchProjectTypes searches project types with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchProjectTypes(ctx context.Context, params ProjectTypeSearchParams) ([]ProjectTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
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
	result := make([]ProjectTypeEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectTypeEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectTypes: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListProjectTypesPage retrieves a single page of ProjectTypeEntity.
func (c *Client) ListProjectTypesPage(ctx context.Context, page, perPage int) (*PageResult[ProjectTypeEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ProjectTypeEntity](c, ctx, makeReq, page, perPage)
}

// ListProjectTypesRaw retrieves all project types and returns the raw HTTP
// response bodies merged across pages as a single JSON array.
// Unlike ListProjectTypes, the returned bytes are byte-preserving: each
// element JSON is exactly what the BOARD API emitted, enabling strict field
// diff in E2E tests to detect keys that are not mapped to ProjectTypeEntity.
//
// Intended for E2E strict field diff; regular callers should use ListProjectTypes.
func (c *Client) ListProjectTypesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetProjectTypeRaw retrieves a single project type and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetProjectType.
func (c *Client) GetProjectTypeRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchProjectTypesRaw retrieves project types matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListProjectTypesRaw.
//
// Intended for E2E strict field diff; regular callers should use SearchProjectTypes.
func (c *Client) SearchProjectTypesRaw(ctx context.Context, params ProjectTypeSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
