package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// AccountingTypeEntity は BOARD API の勘定科目エンティティ。
// GET /v1/accounting_types レスポンスの1要素に対応。
type AccountingTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// AccountingTypeSearchParams は SearchAccountingTypes のパラメータ。
type AccountingTypeSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListAccountingTypes は全勘定科目を取得する。
// ページネーションは ListAll により自動処理される。
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

// GetAccountingType は指定 ID の勘定科目を取得する。
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

// SearchAccountingTypes は条件付きで勘定科目を検索する。
// ページネーションは ListAll により自動処理される。
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
