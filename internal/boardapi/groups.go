package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// GroupEntity is a BOARD API group entity.
// Corresponds to one element in the GET /v1/groups response.
type GroupEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// GroupSearchParams is the parameter for SearchGroups.
type GroupSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListGroups retrieves all groups.
// Pagination is automatically handled by ListAll.
func (c *Client) ListGroups(ctx context.Context) ([]GroupEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/groups", nil)
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
	result := make([]GroupEntity, 0, len(items))
	for _, raw := range items {
		var x GroupEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListGroups: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetGroup retrieves the group with the specified ID.
func (c *Client) GetGroup(ctx context.Context, id int) (*GroupEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/groups/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x GroupEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetGroup: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchGroups searches groups with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchGroups(ctx context.Context, params GroupSearchParams) ([]GroupEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/groups", nil)
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
	result := make([]GroupEntity, 0, len(items))
	for _, raw := range items {
		var x GroupEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchGroups: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListGroupsPage retrieves a single page of groups.
func (c *Client) ListGroupsPage(ctx context.Context, page, perPage int) (*PageResult[GroupEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/groups", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[GroupEntity](c, ctx, makeReq, page, perPage)
}
