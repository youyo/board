package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectEntity は BOARD API の案件エンティティ。
// GET /v1/projects レスポンスの1要素に対応。
type ProjectEntity struct {
	ID        int    `json:"id"`
	ClientID  int    `json:"client_id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	StartDate string `json:"start_date"` // ISO 8601 date
	EndDate   string `json:"end_date"`   // ISO 8601 date
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// ProjectSearchParams は SearchProjects のパラメータ。
type ProjectSearchParams struct {
	ClientID      int
	Name          string
	Status        string
	UpdatedAtFrom string
}

// ListProjects は全案件を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListProjects(ctx context.Context) ([]ProjectEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
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
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetProject は指定 ID の案件を取得する。
func (c *Client) GetProject(ctx context.Context, id int) (*ProjectEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProject: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchProjects は条件付きで案件を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchProjects(ctx context.Context, params ProjectSearchParams) ([]ProjectEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
		}
		if params.Name != "" {
			q.Set("name", params.Name)
		}
		if params.Status != "" {
			q.Set("status", params.Status)
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
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
