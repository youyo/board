package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectCostEntity は BOARD API の ProjectCost エンティティ。
// GET /v1/project_costs および GET /v1/project_costs/{id} の実レスポンスに
// 完全一致する 8 フィールド定義。
//
// 実データ根拠: tmp/e2e-artifacts/project_costs_33291004.json
// 概念: プロジェクト原価台帳の行（個別支払い記録）
type ProjectCostEntity struct {
	ID          int     `json:"id"`
	ProjectID   int     `json:"project_id"`
	Description string  `json:"description"`   // 支払い内容の説明
	Cost        int     `json:"cost"`           // 金額（整数）
	InvoiceDate *string `json:"invoice_date"`   // 請求日（ISO date）、null 可
	PaymentDate *string `json:"payment_date"`   // 支払日（ISO date）、null 可
	UpdatedAt   string  `json:"updated_at"`     // ISO 8601
	CreatedAt   string  `json:"created_at"`     // ISO 8601
}

// ProjectCostSearchParams is the parameter for SearchProjectCosts.
type ProjectCostSearchParams struct {
	ProjectID int
}

// ListProjectCosts retrieves all project costs.
// Pagination is automatically handled by ListAll.
func (c *Client) ListProjectCosts(ctx context.Context) ([]ProjectCostEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
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
	result := make([]ProjectCostEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectCostEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectCosts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetProjectCost retrieves the project cost with the specified ID.
func (c *Client) GetProjectCost(ctx context.Context, id int) (*ProjectCostEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_costs/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectCostEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectCost: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchProjectCosts searches project costs with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchProjectCosts(ctx context.Context, params ProjectCostSearchParams) ([]ProjectCostEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ProjectID != 0 {
			q.Set("project_id", strconv.Itoa(params.ProjectID))
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectCostEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectCostEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectCosts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListProjectCostsPage retrieves a single page of project costs.
func (c *Client) ListProjectCostsPage(ctx context.Context, page, perPage int) (*PageResult[ProjectCostEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ProjectCostEntity](c, ctx, makeReq, page, perPage)
}

// ListProjectCostsRaw retrieves all project costs and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Unlike
// ListProjectCosts, the returned bytes are byte-preserving: each element JSON
// is exactly what the BOARD API emitted, enabling strict field diff in E2E
// tests to detect keys that are not mapped to ProjectCostEntity.
//
// Intended for E2E strict field diff; regular callers should use
// ListProjectCosts.
func (c *Client) ListProjectCostsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectCostsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetProjectCostRaw retrieves a single project cost and returns the raw HTTP
// response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use
// GetProjectCost.
func (c *Client) GetProjectCostRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_costs/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchProjectCostsRaw retrieves project costs matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as
// a single JSON array. Same byte-preserving guarantee as ListProjectCostsRaw.
//
// Unlike contacts (client_id/name/email) or client_branches (client_id/name),
// project_costs exposes only a single hierarchical filter `project_id`.
//
// Intended for E2E strict field diff; regular callers should use
// SearchProjectCosts.
func (c *Client) SearchProjectCostsRaw(ctx context.Context, params ProjectCostSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ProjectID != 0 {
			q.Set("project_id", strconv.Itoa(params.ProjectID))
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectCostsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
