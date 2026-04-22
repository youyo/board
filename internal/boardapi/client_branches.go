package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ClientBranchEntity は BOARD API の顧客支社エンティティ。
// GET /v1/client_branches のレスポンス 1 要素に対応する。
// 実 API レスポンス（tmp/e2e-artifacts/client_branches_*.json）に基づき M39 で全面再設計。
type ClientBranchEntity struct {
	ID         int        `json:"id"`
	Client     *ClientRef `json:"client"` // nested 構造: {id, name, name_disp, custom_no}
	Name       string     `json:"name"`
	Zip        string     `json:"zip"`
	Pref       string     `json:"pref"`
	Address1   string     `json:"address1"`
	Address2   string     `json:"address2"`
	Tel        *string    `json:"tel"` // null 可
	Fax        *string    `json:"fax"` // null 可
	ArchiveFlg int        `json:"archive_flg"`
	CreatedAt  string     `json:"created_at"` // ISO 8601
	UpdatedAt  string     `json:"updated_at"` // ISO 8601
}

// ClientID は nested Client.ID を返す accessor（後方互換ブリッジ）。
// Client が nil の場合 0 を返す。
func (e ClientBranchEntity) ClientID() int {
	if e.Client == nil {
		return 0
	}
	return e.Client.ID
}

// ClientBranchSearchParams is the parameter for SearchClientBranches.
type ClientBranchSearchParams struct {
	ClientID int
	Name     string
}

// ListClientBranches retrieves all customer branches.
// Pagination is automatically handled by ListAll.
func (c *Client) ListClientBranches(ctx context.Context) ([]ClientBranchEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
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
	result := make([]ClientBranchEntity, 0, len(items))
	for _, raw := range items {
		var x ClientBranchEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListClientBranches: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetClientBranch retrieves the customer branch with the specified ID.
func (c *Client) GetClientBranch(ctx context.Context, id int) (*ClientBranchEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/client_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ClientBranchEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetClientBranch: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchClientBranches searches customer branches with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchClientBranches(ctx context.Context, params ClientBranchSearchParams) ([]ClientBranchEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
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
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]ClientBranchEntity, 0, len(items))
	for _, raw := range items {
		var x ClientBranchEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchClientBranches: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListClientBranchesPage retrieves a single page of customer branches.
func (c *Client) ListClientBranchesPage(ctx context.Context, page, perPage int) (*PageResult[ClientBranchEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ClientBranchEntity](c, ctx, makeReq, page, perPage)
}

// ListClientBranchesRaw retrieves all customer branches and returns the raw
// HTTP response bodies merged across pages as a single JSON array. Unlike
// ListClientBranches, the returned bytes are byte-preserving: each element
// JSON is exactly what the BOARD API emitted, enabling strict field diff in
// E2E tests to detect keys that are not mapped to ClientBranchEntity.
//
// Intended for E2E strict field diff; regular callers should use
// ListClientBranches.
func (c *Client) ListClientBranchesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListClientBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetClientBranchRaw retrieves a single customer branch and returns the raw
// HTTP response body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use
// GetClientBranch.
func (c *Client) GetClientBranchRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/client_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchClientBranchesRaw retrieves customer branches matching the given
// search parameters and returns the raw HTTP response bodies merged across
// pages as a single JSON array. Same byte-preserving guarantee as
// ListClientBranchesRaw.
//
// Intended for E2E strict field diff; regular callers should use
// SearchClientBranches.
func (c *Client) SearchClientBranchesRaw(ctx context.Context, params ClientBranchSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
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
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchClientBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
