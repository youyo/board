package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectEntity is a BOARD API project entity.
// Corresponds to one element in the GET /v1/projects response.
type ProjectEntity struct {
	ID        int    `json:"id"`
	ClientID  int    `json:"client_id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	StartDate string `json:"start_date"` // ISO 8601 date
	EndDate   string `json:"end_date"`   // ISO 8601 date
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
	// Optional document summaries (only populated when response_group is specified)
	// estimate と order は API が単一オブジェクトで返す（単数形キー）。
	Estimate *DocumentSummary `json:"estimate,omitempty"`
	Order    *DocumentSummary `json:"order,omitempty"`
	// delivery / invoice / receipt は API が複数形配列で返す。
	// 後方互換のため単数形フィールドも残すが、delivery/receipt の参照は Deliveries/Receipts を使うこと。
	Delivery   *DocumentSummary  `json:"delivery,omitempty"`
	Deliveries []DocumentSummary `json:"deliveries,omitempty"`
	Invoice    *DocumentSummary  `json:"invoice,omitempty"`
	Invoices   []DocumentSummary `json:"invoices,omitempty"`
	Receipt    *DocumentSummary  `json:"receipt,omitempty"`
	Receipts   []DocumentSummary `json:"receipts,omitempty"`
}

// ProjectSearchParams is the parameter for SearchProjects.
type ProjectSearchParams struct {
	ClientID      int
	Name          string
	Status        string
	UpdatedAtFrom string
	ResponseGroup string
}

// ListProjects retrieves all projects.
// Pagination is automatically handled by ListAll.
func (c *Client) ListProjects(ctx context.Context) ([]ProjectEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
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
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetProject retrieves the project with the specified ID.
func (c *Client) GetProject(ctx context.Context, id int) (*ProjectEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProject: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// GetProjectWithGroup retrieves the project with the specified ID and response_group.
// responseGroup can be "estimate", "order", "delivery", "invoice", "receipt", or "all".
// If responseGroup is empty, behaves like GetProject.
func (c *Client) GetProjectWithGroup(ctx context.Context, id int, responseGroup string) (*ProjectEntity, error) {
	path := fmt.Sprintf("/v1/projects/%d", id)
	if responseGroup != "" {
		path += "?response_group=" + responseGroup
	}
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectWithGroup: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchProjects searches projects with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchProjects(ctx context.Context, params ProjectSearchParams) ([]ProjectEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
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
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		if params.ResponseGroup != "" {
			q.Set("response_group", params.ResponseGroup)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListProjectsPage retrieves a single page of projects.
func (c *Client) ListProjectsPage(ctx context.Context, page, perPage int) (*PageResult[ProjectEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ProjectEntity](c, ctx, makeReq, page, perPage)
}

// ListProjectsRaw retrieves all projects and returns the raw HTTP response
// bodies merged across pages as a single JSON array. Unlike ListProjects, the
// returned bytes are byte-preserving: each element JSON is exactly what the
// BOARD API emitted, enabling strict field diff in E2E tests to detect keys
// that are not mapped to ProjectEntity.
//
// Intended for E2E strict field diff; regular callers should use ListProjects.
func (c *Client) ListProjectsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetProjectRaw retrieves a single project and returns the raw HTTP response
// body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetProject.
func (c *Client) GetProjectRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// GetProjectWithGroupRaw retrieves a single project with response_group and
// returns the raw HTTP response body byte-for-byte.
//
// responseGroup can be "estimate", "order", "delivery", "invoice", "receipt",
// or "all". If responseGroup is empty, behaves like GetProjectRaw (no
// response_group query is appended).
//
// The query is concatenated directly to the path via "?response_group=<value>"
// to mirror GetProjectWithGroup's existing implementation; BOARD API has been
// validated to parse this form. Raw variant is exposed for E2E strict field
// diff on DocumentSummary sub-entities (estimate/order/delivery/invoice/
// receipt) which appear only when response_group is specified.
//
// Intended for E2E strict field diff; regular callers should use
// GetProjectWithGroup.
func (c *Client) GetProjectWithGroupRaw(ctx context.Context, id int, responseGroup string) ([]byte, error) {
	path := fmt.Sprintf("/v1/projects/%d", id)
	if responseGroup != "" {
		path += "?response_group=" + responseGroup
	}
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchProjectsRaw retrieves projects matching the given search parameters
// and returns the raw HTTP response bodies merged across pages as a single
// JSON array. Same byte-preserving guarantee as ListProjectsRaw.
//
// ProjectSearchParams exposes 5 filters (ClientID, Name, Status, UpdatedAtFrom,
// ResponseGroup) — the richest surface across M02-M12. Note that the BOARD
// API has been observed to ignore the `name` filter across 7 consecutive
// milestones (M03/M04/M06/M08/M09/M10/M12), so the Name value in Search only
// exercises request encoding, not server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use
// SearchProjects.
func (c *Client) SearchProjectsRaw(ctx context.Context, params ProjectSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
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
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		if params.ResponseGroup != "" {
			q.Set("response_group", params.ResponseGroup)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
