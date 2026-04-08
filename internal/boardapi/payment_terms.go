package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// PaymentTermEntity は BOARD API の支払条件エンティティ。
// GET /v1/payment_terms レスポンスの1要素に対応。
type PaymentTermEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PaymentTermSearchParams は SearchPaymentTerms のパラメータ。
type PaymentTermSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListPaymentTerms は全支払条件を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListPaymentTerms(ctx context.Context) ([]PaymentTermEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
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
	result := make([]PaymentTermEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentTermEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentTerms: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetPaymentTerm は指定 ID の支払条件を取得する。
func (c *Client) GetPaymentTerm(ctx context.Context, id int) (*PaymentTermEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payment_terms/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x PaymentTermEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPaymentTerm: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchPaymentTerms は条件付きで支払条件を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchPaymentTerms(ctx context.Context, params PaymentTermSearchParams) ([]PaymentTermEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
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
	result := make([]PaymentTermEntity, 0, len(items))
	for _, raw := range items {
		var x PaymentTermEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchPaymentTerms: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
