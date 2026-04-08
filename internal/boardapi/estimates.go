package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// EstimateEntity は BOARD API の見積エンティティ。
// GET /v1/estimates レスポンスの1要素に対応。
type EstimateEntity struct {
	ID             int     `json:"id"`
	ClientID       int     `json:"client_id"`
	ProjectID      int     `json:"project_id"`
	Title          string  `json:"title"`
	TotalAmount    float64 `json:"total_amount"`
	Status         string  `json:"status"`
	EstimateDate   string  `json:"estimate_date"`   // ISO 8601 date
	ExpirationDate string  `json:"expiration_date"` // ISO 8601 date
	Memo           string  `json:"memo"`
	UpdatedAt      string  `json:"updated_at"` // ISO 8601
	CreatedAt      string  `json:"created_at"` // ISO 8601
}

// EstimateSearchParams は SearchEstimates のパラメータ。
type EstimateSearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// ListEstimates は全見積を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListEstimates(ctx context.Context) ([]EstimateEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/estimates", nil)
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
	result := make([]EstimateEntity, 0, len(items))
	for _, raw := range items {
		var x EstimateEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListEstimates: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetEstimate は指定 ID の見積を取得する。
func (c *Client) GetEstimate(ctx context.Context, id int) (*EstimateEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/estimates/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x EstimateEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetEstimate: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchEstimates は条件付きで見積を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchEstimates(ctx context.Context, params EstimateSearchParams) ([]EstimateEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/estimates", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
		}
		if params.ProjectID != 0 {
			q.Set("project_id", strconv.Itoa(params.ProjectID))
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
	result := make([]EstimateEntity, 0, len(items))
	for _, raw := range items {
		var x EstimateEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchEstimates: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
