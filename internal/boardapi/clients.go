package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ClientEntity は BOARD API の顧客エンティティ。
// GET /v1/clients レスポンスの1要素に対応。
// 注意: boardapi.Client は HTTP クライアント型のため、顧客エンティティは ClientEntity と命名。
type ClientEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// ClientSearchParams は SearchClients のパラメータ。
type ClientSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListClients は全顧客を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListClients(ctx context.Context) ([]ClientEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
	result := make([]ClientEntity, 0, len(items))
	for _, raw := range items {
		var x ClientEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListClients: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetClient は指定 ID の顧客を取得する。
func (c *Client) GetClient(ctx context.Context, id int) (*ClientEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/clients/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ClientEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetClient: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchClients は条件付きで顧客を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchClients(ctx context.Context, params ClientSearchParams) ([]ClientEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
	result := make([]ClientEntity, 0, len(items))
	for _, raw := range items {
		var x ClientEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchClients: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
