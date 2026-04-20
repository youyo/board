package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ContactEntity is a BOARD API contact entity.
// Corresponds to one element in the GET /v1/contacts response.
type ContactEntity struct {
	ID             int    `json:"id"`
	ClientID       int    `json:"client_id"`
	ClientBranchID int    `json:"client_branch_id"`
	Name           string `json:"name"`
	NameKana       string `json:"name_kana"`
	LastName       string `json:"last_name"`
	FirstName      string `json:"first_name"`
	HonorificTitle string `json:"honorific_title"`
	Title          string `json:"title"`
	Department     string `json:"department"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Note           string `json:"note"`
	Memo           string `json:"memo"`
	ArchiveFlg     int    `json:"archive_flg"`
	UpdatedAt      string `json:"updated_at"` // ISO 8601
	CreatedAt      string `json:"created_at"` // ISO 8601
}

// DisplayName returns a human-readable name.
// Prefers Name if set, otherwise combines LastName + FirstName.
func (c ContactEntity) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	switch {
	case c.LastName != "" && c.FirstName != "":
		return c.LastName + " " + c.FirstName
	case c.LastName != "":
		return c.LastName
	case c.FirstName != "":
		return c.FirstName
	default:
		return ""
	}
}

// ContactSearchParams is the parameter for SearchContacts.
type ContactSearchParams struct {
	ClientID int
	Name     string
	Email    string
}

// ListContacts retrieves all contacts.
// Pagination is automatically handled by ListAll.
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

// GetContact retrieves the contact with the specified ID.
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

// SearchContacts searches contacts with the given conditions.
// Pagination is automatically handled by ListAll.
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

// ListContactsPage retrieves a single page of contacts.
func (c *Client) ListContactsPage(ctx context.Context, page, perPage int) (*PageResult[ContactEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/contacts", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ContactEntity](c, ctx, makeReq, page, perPage)
}

// ListContactsRaw retrieves all contacts and returns the raw HTTP response
// bodies merged across pages as a single JSON array. Unlike ListContacts, the
// returned bytes are byte-preserving: each element JSON is exactly what the
// BOARD API emitted, enabling strict field diff in E2E tests to detect keys
// that are not mapped to ContactEntity.
//
// Intended for E2E strict field diff; regular callers should use ListContacts.
func (c *Client) ListContactsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetContactRaw retrieves a single contact and returns the raw HTTP response
// body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetContact.
func (c *Client) GetContactRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchContactsRaw retrieves contacts matching the given search parameters
// and returns the raw HTTP response bodies merged across pages as a single
// JSON array. Same byte-preserving guarantee as ListContactsRaw.
//
// Intended for E2E strict field diff; regular callers should use
// SearchContacts.
func (c *Client) SearchContactsRaw(ctx context.Context, params ContactSearchParams, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
