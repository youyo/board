package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// VendorContactEntity は BOARD API の発注先担当者エンティティ。
// GET /v1/vendor_contacts レスポンスの1要素に対応。
type VendorContactEntity struct {
	ID             int    `json:"id"`
	VendorID       int    `json:"vendor_id"`
	VendorBranchID int    `json:"vendor_branch_id"`
	Name           string `json:"name"`
	NameKana       string `json:"name_kana"`
	Title          string `json:"title"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Memo           string `json:"memo"`
	UpdatedAt      string `json:"updated_at"` // ISO 8601
	CreatedAt      string `json:"created_at"` // ISO 8601
}

// VendorContactSearchParams は SearchVendorContacts のパラメータ。
type VendorContactSearchParams struct {
	VendorID int
	Name     string
	Email    string
}

// ListVendorContacts は全発注先担当者を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListVendorContacts(ctx context.Context) ([]VendorContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/vendor_contacts", nil)
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
	result := make([]VendorContactEntity, 0, len(items))
	for _, raw := range items {
		var x VendorContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetVendorContact は指定 ID の発注先担当者を取得する。
func (c *Client) GetVendorContact(ctx context.Context, id int) (*VendorContactEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/vendor_contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x VendorContactEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendorContact: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchVendorContacts は条件付きで発注先担当者を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchVendorContacts(ctx context.Context, params VendorContactSearchParams) ([]VendorContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/vendor_contacts", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
		if params.Name != "" {
			q.Set("name", params.Name)
		}
		if params.Email != "" {
			q.Set("email", params.Email)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]VendorContactEntity, 0, len(items))
	for _, raw := range items {
		var x VendorContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
