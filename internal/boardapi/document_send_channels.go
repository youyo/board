package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// DocumentSendChannelEntity is a BOARD API document send channel entity.
// Corresponds to one element in the GET /v1/document_send_channels response.
type DocumentSendChannelEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// DocumentSendChannelSearchParams is the parameter for SearchDocumentSendChannels.
type DocumentSendChannelSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListDocumentSendChannels retrieves all document send channels.
// Pagination is automatically handled by ListAll.
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

// GetDocumentSendChannel retrieves the document send channel with the specified ID.
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

// SearchDocumentSendChannels searches document send channels with the given conditions.
// Pagination is automatically handled by ListAll.
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
