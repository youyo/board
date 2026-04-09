package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// UserEntity is a BOARD API user entity.
// Corresponds to one element in the GET /v1/users response.
type UserEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// UserSearchParams is the parameter for SearchUsers.
type UserSearchParams struct {
	Name          string
	Email         string
	UpdatedAtFrom string
}

// ListUsers retrieves all users.
// Pagination is automatically handled by ListAll.
func (c *Client) ListUsers(ctx context.Context) ([]UserEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/users", nil)
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
	result := make([]UserEntity, 0, len(items))
	for _, raw := range items {
		var x UserEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListUsers: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetUser retrieves the user with the specified ID.
func (c *Client) GetUser(ctx context.Context, id int) (*UserEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x UserEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetUser: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchUsers searches users with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchUsers(ctx context.Context, params UserSearchParams) ([]UserEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/users", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.Name != "" {
			q.Set("name", params.Name)
		}
		if params.Email != "" {
			q.Set("email", params.Email)
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
	result := make([]UserEntity, 0, len(items))
	for _, raw := range items {
		var x UserEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchUsers: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListUsersPage retrieves a single page of users.
func (c *Client) ListUsersPage(ctx context.Context, page, perPage int) (*PageResult[UserEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/users", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[UserEntity](c, ctx, makeReq, page, perPage)
}
