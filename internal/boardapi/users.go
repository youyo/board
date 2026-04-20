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
	ID           int    `json:"id"`
	Name         string `json:"name"`
	LastName     string `json:"last_name"`
	FirstName    string `json:"first_name"`
	Email        string `json:"email"`
	RoleID       int    `json:"role_id"`
	RoleName     string `json:"role_name"`
	LastSignInAt string `json:"last_sign_in_at"` // ISO 8601
	ValidFlg     int    `json:"valid_flg"`
	UpdatedAt    string `json:"updated_at"` // ISO 8601
	CreatedAt    string `json:"created_at"` // ISO 8601
}

// DisplayName returns a human-readable name.
// Prefers Name if set, otherwise combines LastName + FirstName.
func (u UserEntity) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	switch {
	case u.LastName != "" && u.FirstName != "":
		return u.LastName + " " + u.FirstName
	case u.LastName != "":
		return u.LastName
	case u.FirstName != "":
		return u.FirstName
	default:
		return ""
	}
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

// ListUsersRaw retrieves all users and returns the raw HTTP response bodies
// merged across pages as a single JSON array. Unlike ListUsers, the returned
// bytes are byte-preserving: each element JSON is exactly what the BOARD API
// emitted, enabling strict field diff in E2E tests to detect keys that are not
// mapped to UserEntity.
//
// Intended for E2E strict field diff; regular callers should use ListUsers.
func (c *Client) ListUsersRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListUsersRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetUserRaw retrieves a single user and returns the raw HTTP response body
// byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetUser.
func (c *Client) GetUserRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchUsersRaw retrieves users matching the given search parameters and
// returns the raw HTTP response bodies merged across pages as a single JSON
// array. Same byte-preserving guarantee as ListUsersRaw.
//
// Intended for E2E strict field diff; regular callers should use SearchUsers.
func (c *Client) SearchUsersRaw(ctx context.Context, params UserSearchParams, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchUsersRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
