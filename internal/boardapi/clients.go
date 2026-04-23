package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	Title                        *string `json:"title"`    // null 可
	Zip                          *string `json:"zip"`      // null 可
	Pref                         *string `json:"pref"`     // null 可
	Address1                     *string `json:"address1"` // null 可
	Address2                     *string `json:"address2"` // null 可
	Tel                          *string `json:"tel"`      // null 可
	Fax                          *string `json:"fax"`      // null 可
	PaymentTermID                int     `json:"payment_term_id"`
	PaymentTermName              string  `json:"payment_term_name"`
	CompanyNumber                *string `json:"company_number"`        // null 可
	InvoiceSystemNumber          *string `json:"invoice_system_number"` // null 可
	InvoiceSystemNumberValidated bool    `json:"invoice_system_number_validated"`
	InvoiceSystemIssuerType      int     `json:"invoice_system_issuer_type"`
	InvoiceSystemIssuerTypeName  string  `json:"invoice_system_issuer_type_name"`

	// Get 限定フィールド（List では返却されない）
	AccountingCode        *string  `json:"accounting_code,omitempty"`
	ArchiveFlg            int      `json:"archive_flg,omitempty"`
	BankChargeToClientFlg int      `json:"bank_charge_to_client_flg,omitempty"`
	BasicAgreementFlg     int      `json:"basic_agreement_flg,omitempty"`
	CC                    *string  `json:"cc,omitempty"`
	CompanyBankID         *int     `json:"company_bank_id,omitempty"`
	CompanyBankName       *string  `json:"company_bank_name,omitempty"`
	CustomNo              *string  `json:"custom_no,omitempty"` // 旧 Code の代替
	DocumentSendType      int      `json:"document_send_type,omitempty"`
	DocumentSendTypeName  string   `json:"document_send_type_name,omitempty"`
	NdaFlg                int      `json:"nda_flg,omitempty"`
	Note                  *string  `json:"note,omitempty"` // 旧 Memo の代替
	Tags                  []string `json:"tags,omitempty"`
	To                    *string  `json:"to,omitempty"`
}

// ClientListOptions corresponds to BOARD API's GET /v1/clients query
// parameters (Ransack-style). Zero values are not sent to the API so a zero
// ClientListOptions{} yields an unfiltered list request.
//
// Introduced in M50 as the Phase L clients-pilot migration target.
// Replaces the pre-M50 ClientSearchParams struct, which had ad-hoc Name /
// UpdatedAtFrom fields that did not match BOARD API's Ransack naming.
type ClientListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）。
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// clients 専用
	NameCont              string   // 顧客名部分一致（Ransack _cont）
	NameDispCont          string   // 顧客略称部分一致
	InvoiceSystemNumberEq string   // 適格請求書登録番号 完全一致
	CustomNoEq            string   // 顧客番号 完全一致
	Tags                  []string // タグ
	ResponseGroup         string   // "small" (既定) / "large"
}

// buildClientsQuery builds the Ransack-style query string for
// GET /v1/clients based on the given options and page.
func buildClientsQuery(opts ClientListOptions, page, perPage int) string {
	qb := NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		StrCont("name_disp", opts.NameDispCont).
		StrEq("invoice_system_number", opts.InvoiceSystemNumberEq).
		StrEq("custom_no", opts.CustomNoEq).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Tags(opts.Tags).
		ResponseGroup(opts.ResponseGroup)
	return qb.Encode()
}

// ListClients retrieves customers filtered by the given options.
// Pagination is handled internally by ListAllWithResult; metadata (total
// count, page, rate limits, ETag) is surfaced via the returned *ListResult.
//
// Pass ClientListOptions{} for an unfiltered list of all customers.
func (c *Client) ListClients(ctx context.Context, opts ClientListOptions) (*ListResult[ClientEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildClientsQuery(opts, page, pp)
		return req, nil
	}
	var listOpts []ListAllOption
	if perPage > 0 {
		listOpts = append(listOpts, WithPerPage(perPage))
	}
	raw, err := c.ListAllWithResult(ctx, makeReq, listOpts...)
	if err != nil {
		return nil, err
	}
	items := make([]ClientEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ClientEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListClients: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ClientEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetClient retrieves the customer with the specified ID and returns an
// ItemResult carrying the entity together with response metadata (ETag,
// rate limits, Last-Modified).
func (c *Client) GetClient(ctx context.Context, id int) (*ItemResult[ClientEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/clients/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ClientEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetClient: unmarshal: " + err.Error()}
	}
	return &ItemResult[ClientEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListClientsRaw retrieves customers filtered by the given options and
// returns the raw HTTP response bodies merged across pages as a single
// JSON array, along with the final page's response headers.
//
// The returned bytes are byte-preserving: each element JSON is exactly what
// the BOARD API emitted, enabling strict field diff in E2E tests to detect
// unmapped keys. Intended for E2E strict field diff; regular callers should
// use ListClients.
func (c *Client) ListClientsRaw(ctx context.Context, opts ClientListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/clients", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildClientsQuery(opts, page, pp)
		return req, nil
	}
	var listOpts []ListAllOption
	if perPage > 0 {
		listOpts = append(listOpts, WithPerPage(perPage))
	}
	raw, err := c.ListAllWithResult(ctx, makeReq, listOpts...)
	if err != nil {
		return nil, nil, err
	}
	out, err := json.Marshal(raw.Items)
	if err != nil {
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListClientsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetClientRaw retrieves a single customer and returns the raw HTTP response
// body byte-for-byte, along with the response headers.
//
// Intended for E2E strict field diff; regular callers should use GetClient.
func (c *Client) GetClientRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/clients/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
