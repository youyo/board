package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectCostEntity は BOARD API の案件原価エンティティ。
// GET /v1/project_costs レスポンスの1要素に対応。
type ProjectCostEntity struct {
	ID        int     `json:"id"`
	ProjectID int     `json:"project_id"`
	Name      string  `json:"name"`
	CostType  string  `json:"cost_type"`
	Amount    float64 `json:"amount"`
	Memo      string  `json:"memo"`
	UpdatedAt string  `json:"updated_at"` // ISO 8601
	CreatedAt string  `json:"created_at"` // ISO 8601
}

// ProjectCostSearchParams は SearchProjectCosts のパラメータ。
type ProjectCostSearchParams struct {
	ProjectID int
}

// ListProjectCosts は全案件原価を取得する。
// ページネーションは ListAll により自動処理される。
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

// GetProjectCost は指定 ID の案件原価を取得する。
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

// SearchProjectCosts は条件付きで案件原価を検索する。
// ページネーションは ListAll により自動処理される。
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
