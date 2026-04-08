package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// DocumentSendChannelEntity は BOARD API の書類送付方法エンティティ。
// GET /v1/document_send_channels レスポンスの1要素に対応。
type DocumentSendChannelEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// DocumentSendChannelSearchParams は SearchDocumentSendChannels のパラメータ。
type DocumentSendChannelSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListDocumentSendChannels は全書類送付方法を取得する。
// ページネーションは ListAll により自動処理される。
func (c *Client) ListDocumentSendChannels(ctx context.Context) ([]DocumentSendChannelEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/document_send_channels", nil)
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
	result := make([]DocumentSendChannelEntity, 0, len(items))
	for _, raw := range items {
		var x DocumentSendChannelEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListDocumentSendChannels: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetDocumentSendChannel は指定 ID の書類送付方法を取得する。
func (c *Client) GetDocumentSendChannel(ctx context.Context, id int) (*DocumentSendChannelEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/document_send_channels/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x DocumentSendChannelEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetDocumentSendChannel: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchDocumentSendChannels は条件付きで書類送付方法を検索する。
// ページネーションは ListAll により自動処理される。
func (c *Client) SearchDocumentSendChannels(ctx context.Context, params DocumentSendChannelSearchParams) ([]DocumentSendChannelEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/document_send_channels", nil)
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
	result := make([]DocumentSendChannelEntity, 0, len(items))
	for _, raw := range items {
		var x DocumentSendChannelEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchDocumentSendChannels: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}
