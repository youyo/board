package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ClientEntity は BOARD API の顧客エンティティ。
// GET /v1/clients および GET /v1/clients/{id} のレスポンスに対応する。
// 実 API レスポンス（tmp/e2e-artifacts/clients_51285623.json）に基づき M43 で全面再設計。
//
// 注意: ClientEntity と命名しているのは boardapi.Client が HTTP クライアント型であるため。
//
// フィールド分類:
//   - 共通（List/Search/Get 全て）: ID, Name, NameDisp, PaymentTermID, PaymentTermName,
//     InvoiceSystemIssuerType, InvoiceSystemIssuerTypeName,
//     InvoiceSystemNumberValidated, UpdatedAt, CreatedAt
//   - Get 限定（omitempty + *): AccountingCode, ArchiveFlg, BankChargeToClientFlg,
//     BasicAgreementFlg, CC, CompanyBankID, CompanyBankName, CustomNo,
//     DocumentSendType, DocumentSendTypeName, NdaFlg, Note, Tags, To
//   - nullable 共通: Title, Zip, Pref, Address1, Address2, Tel, Fax,
//     CompanyNumber, InvoiceSystemNumber
type ClientEntity struct {
	// 既存維持（4）
	ID        int    `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601

	// 共通フィールド（List/Search/Get 全て返却）
	NameDisp                     string  `json:"name_disp"`
	Title                        *string `json:"title"`                          // null 可
	Zip                          *string `json:"zip"`                            // null 可
	Pref                         *string `json:"pref"`                           // null 可
	Address1                     *string `json:"address1"`                       // null 可
	Address2                     *string `json:"address2"`                       // null 可
	Tel                          *string `json:"tel"`                            // null 可
	Fax                          *string `json:"fax"`                            // null 可
	PaymentTermID                int     `json:"payment_term_id"`
	PaymentTermName              string  `json:"payment_term_name"`
	CompanyNumber                *string `json:"company_number"`                 // null 可
	InvoiceSystemNumber          *string `json:"invoice_system_number"`          // null 可
	InvoiceSystemNumberValidated bool    `json:"invoice_system_number_validated"`
	InvoiceSystemIssuerType      int     `json:"invoice_system_issuer_type"`
	InvoiceSystemIssuerTypeName  string  `json:"invoice_system_issuer_type_name"`

	// Get 限定フィールド（List では返却されない）
	AccountingCode       *string  `json:"accounting_code,omitempty"`
	ArchiveFlg           int      `json:"archive_flg,omitempty"`
	BankChargeToClientFlg int     `json:"bank_charge_to_client_flg,omitempty"`
	BasicAgreementFlg    int      `json:"basic_agreement_flg,omitempty"`
	CC                   *string  `json:"cc,omitempty"`
	CompanyBankID        *int     `json:"company_bank_id,omitempty"`
	CompanyBankName      *string  `json:"company_bank_name,omitempty"`
	CustomNo             *string  `json:"custom_no,omitempty"`             // 旧 Code の代替
	DocumentSendType     int      `json:"document_send_type,omitempty"`
	DocumentSendTypeName string   `json:"document_send_type_name,omitempty"`
	NdaFlg               int      `json:"nda_flg,omitempty"`
	Note                 *string  `json:"note,omitempty"`                  // 旧 Memo の代替
	Tags                 []string `json:"tags,omitempty"`
	To                   *string  `json:"to,omitempty"`
}

// ClientSearchParams is the parameter for SearchClients.
type ClientSearchParams struct {
	Name          string
	UpdatedAtFrom string
}

// ListClients retrieves all customers.
// Pagination is automatically handled by ListAll.
func (c *Client) ListClients(ctx context.Context) ([]ClientEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
	result := make([]ClientEntity, 0, len(items))
	for _, raw := range items {
		var x ClientEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListClients: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetClient retrieves the customer with the specified ID.
func (c *Client) GetClient(ctx context.Context, id int) (*ClientEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/clients/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ClientEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetClient: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchClients searches customers with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchClients(ctx context.Context, params ClientSearchParams) ([]ClientEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
	result := make([]ClientEntity, 0, len(items))
	for _, raw := range items {
		var x ClientEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchClients: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListClientsPage retrieves a single page of customers.
func (c *Client) ListClientsPage(ctx context.Context, page, perPage int) (*PageResult[ClientEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ClientEntity](c, ctx, makeReq, page, perPage)
}

// ListClientsRaw retrieves all customers and returns the raw HTTP response
// bodies merged across pages as a single JSON array. Unlike ListClients, the
// returned bytes are byte-preserving: each element JSON is exactly what the
// BOARD API emitted, enabling strict field diff in E2E tests to detect keys
// that are not mapped to ClientEntity.
//
// Intended for E2E strict field diff; regular callers should use ListClients.
func (c *Client) ListClientsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListClientsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetClientRaw retrieves a single customer and returns the raw HTTP response
// body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetClient.
func (c *Client) GetClientRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/clients/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// SearchClientsRaw retrieves customers matching the given search parameters
// and returns the raw HTTP response bodies merged across pages as a single
// JSON array. Same byte-preserving guarantee as ListClientsRaw.
//
// Supported filters: Name, UpdatedAtFrom (mirrors ClientSearchParams). Note
// that the BOARD API has been observed to ignore the `name` filter across 6
// consecutive milestones (M03/M04/M06/M08/M09/M10), so the test value is used
// only to exercise request encoding, not to assert server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use SearchClients.
func (c *Client) SearchClientsRaw(ctx context.Context, params ClientSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchClientsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
