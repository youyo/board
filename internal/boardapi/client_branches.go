package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ClientBranchEntity is a BOARD API customer branch entity.
// Corresponds to one element in the GET /v1/client_branches response.
type ClientBranchEntity struct {
	ID         int    `json:"id"`
	ClientID   int    `json:"client_id"`
	Name       string `json:"name"`
	PostalCode string `json:"postal_code"`
	Address    string `json:"address"`
	Phone      string `json:"phone"`
	Fax        string `json:"fax"`
	Memo       string `json:"memo"`
	UpdatedAt  string `json:"updated_at"` // ISO 8601
	CreatedAt  string `json:"created_at"` // ISO 8601
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
