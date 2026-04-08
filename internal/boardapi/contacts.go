package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ContactEntity は BOARD API の担当者エンティティ。
// GET /v1/contacts レスポンスの1要素に対応。
type ContactEntity struct {
	ID             int    `json:"id"`
	ClientID       int    `json:"client_id"`
	ClientBranchID int    `json:"client_branch_id"`
	Name           string `json:"name"`
	NameKana       string `json:"name_kana"`
	Title          string `json:"title"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Memo           string `json:"memo"`
	UpdatedAt      string `json:"updated_at"` // ISO 8601
	CreatedAt      string `json:"created_at"` // ISO 8601
}

// ContactSearchParams は SearchContacts のパラメータ。
type ContactSearchParams struct {
	ClientID int
	Name     string
	Email    string
}

// ListContacts は全担当者を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListContacts(ctx context.Context) ([]ContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/contacts", nil)
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
	result := make([]ContactEntity, 0, len(items))
	for _, raw := range items {
		var x ContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetContact は指定 ID の担当者を取得する。
func (c *Client) GetContact(ctx context.Context, id int) (*ContactEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ContactEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetContact: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchContacts は条件付きで担当者を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchContacts(ctx context.Context, params ContactSearchParams) ([]ContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/contacts", nil)
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
	result := make([]ContactEntity, 0, len(items))
	for _, raw := range items {
		var x ContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
