package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// VendorContactEntity is a BOARD API vendor contact entity.
// Corresponds to one element in the GET /v1/vendor_contacts response.
type VendorContactEntity struct {
	ID             int    `json:"id"`
	VendorID       int    `json:"vendor_id"`
	VendorBranchID int    `json:"vendor_branch_id"`
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
func (vc VendorContactEntity) DisplayName() string {
	if vc.Name != "" {
		return vc.Name
	}
	switch {
	case vc.LastName != "" && vc.FirstName != "":
		return vc.LastName + " " + vc.FirstName
	case vc.LastName != "":
		return vc.LastName
	case vc.FirstName != "":
		return vc.FirstName
	default:
		return ""
	}
}

// VendorContactSearchParams is the parameter for SearchVendorContacts.
type VendorContactSearchParams struct {
	VendorID      int
	Name          string
	Email         string
	UpdatedAtFrom string
}

// ListVendorContacts retrieves all vendor contacts.
// Pagination is automatically handled by ListAll.
func (c *Client) ListVendorContacts(ctx context.Context) ([]VendorContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_contacts", nil)
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
	result := make([]VendorContactEntity, 0, len(items))
	for _, raw := range items {
		var x VendorContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetVendorContact retrieves the vendor contact with the specified ID.
func (c *Client) GetVendorContact(ctx context.Context, id int) (*VendorContactEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x VendorContactEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendorContact: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchVendorContacts searches vendor contacts with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchVendorContacts(ctx context.Context, params VendorContactSearchParams) ([]VendorContactEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_contacts", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.VendorID != 0 {
			q.Set("vendor_id", strconv.Itoa(params.VendorID))
		}
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
	result := make([]VendorContactEntity, 0, len(items))
	for _, raw := range items {
		var x VendorContactEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorContacts: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListVendorContactsPage retrieves a single page of VendorContactEntity.
func (c *Client) ListVendorContactsPage(ctx context.Context, page, perPage int) (*PageResult[VendorContactEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_contacts", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[VendorContactEntity](c, ctx, makeReq, page, perPage)
}
