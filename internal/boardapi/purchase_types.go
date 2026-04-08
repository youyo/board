package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PurchaseTypeEntity は BOARD API の発注区分エンティティ。
// GET /v1/purchase_types レスポンスの1要素に対応。
type PurchaseTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PurchaseTypeSearchParams は SearchPurchaseTypes のパラメータ。
type PurchaseTypeSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListPurchaseTypes は全発注区分を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListPurchaseTypes(ctx context.Context) ([]PurchaseTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/purchase_types", nil)
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

// GetPurchaseType は指定 ID の発注区分を取得する。
func (c *Client) GetPurchaseType(ctx context.Context, id int) (*PurchaseTypeEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/purchase_types/%d", id), nil)
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

// SearchPurchaseTypes は条件付きで発注区分を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchPurchaseTypes(ctx context.Context, params PurchaseTypeSearchParams) ([]PurchaseTypeEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/purchase_types", nil)
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
