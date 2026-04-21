package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// VendorContactEntity は BOARD API の仕入先担当者エンティティ。
// GET /v1/payee_contacts のレスポンス 1 要素に対応する。
// ContactEntity（M40 再設計）と同型のパターンを vendor 側に適用し M42 で全面再設計。
//
// 注意: nested オブジェクトのキー（"vendor"）は未確認（アカウントのデータが 0 件のため）。
// ContactEntity が "client" ネストを使うことを確認済みなので "vendor" と推定する。
// データ投入後の smoke テスト（TestE2E_VendorContacts_*）で Pending Re-verification。
type VendorContactEntity struct {
	ID             int        `json:"id"`
	Vendor         *VendorRef `json:"vendor"`          // nested 構造: {id, name, name_disp, custom_no}（未確認）
	LastName       string     `json:"last_name"`
	FirstName      string     `json:"first_name"`
	HonorificTitle string     `json:"honorific_title"`
	Title          *string    `json:"title"`           // null 可
	Department     *string    `json:"department"`      // null 可
	Email          *string    `json:"email"`           // null 可
	Note           *string    `json:"note"`            // null 可
	ArchiveFlg     int        `json:"archive_flg"`
	CreatedAt      string     `json:"created_at"`      // ISO 8601
	UpdatedAt      string     `json:"updated_at"`      // ISO 8601
}

// VendorID は nested Vendor.ID を返す accessor（後方互換ブリッジ）。
// Vendor が nil の場合 0 を返す。
func (e VendorContactEntity) VendorID() int {
	if e.Vendor == nil {
		return 0
	}
	return e.Vendor.ID
}

// DisplayName は人名を返す。LastName + FirstName を結合する。
// Name フィールドは M42 再設計で廃止（ContactEntity と同様）。
func (e VendorContactEntity) DisplayName() string {
	switch {
	case e.LastName != "" && e.FirstName != "":
		return e.LastName + " " + e.FirstName
	case e.LastName != "":
		return e.LastName
	case e.FirstName != "":
		return e.FirstName
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

// ListVendorContactsRaw retrieves all vendor contacts and returns the raw HTTP
// response bodies merged across pages as a single JSON array. Each element
// preserves the exact byte content returned by the BOARD API.
//
// Note: the real BOARD API path is /v1/payee_contacts.
//
// Intended for E2E strict field diff; regular callers should use
// ListVendorContacts.
func (c *Client) ListVendorContactsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetVendorContactRaw retrieves a single vendor contact and returns the raw HTTP
// response body byte-for-byte.
//
// Note: the real BOARD API path is /v1/payee_contacts/{id}.
//
// Intended for E2E strict field diff; regular callers should use
// GetVendorContact.
func (c *Client) GetVendorContactRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchVendorContactsRaw retrieves vendor contacts matching the given search
// parameters and returns the raw HTTP response bodies merged across pages as a
// single JSON array. Same byte-preserving guarantee as ListVendorContactsRaw.
//
// VendorContactSearchParams exposes 4 filters (VendorID, Name, Email, UpdatedAtFrom).
// Note that the BOARD API has been observed to ignore the `name` filter across
// 9 consecutive milestones (M03-M13), so the Name value in Search only
// exercises request encoding, not server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use
// SearchVendorContacts.
func (c *Client) SearchVendorContactsRaw(ctx context.Context, params VendorContactSearchParams, opts ...ListAllOption) ([]byte, error) {
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchVendorContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
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
