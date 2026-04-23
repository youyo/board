package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// InvoiceEntity は BOARD API の請求書エンティティ。
// GET /v1/invoices および GET /v1/invoices/{id} のレスポンスに対応する。
type InvoiceEntity struct {
	ID          int     `json:"id"`
	ClientID    int     `json:"client_id"`
	ProjectID   int     `json:"project_id"`
	Title       string  `json:"title"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	InvoiceDate string  `json:"invoice_date"` // ISO 8601 date
	DueDate     string  `json:"due_date"`     // ISO 8601 date
	Memo        string  `json:"memo"`
	UpdatedAt   string  `json:"updated_at"` // ISO 8601
	CreatedAt   string  `json:"created_at"` // ISO 8601
}

// InvoiceListOptions は GET /v1/invoices のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信されないため、InvoiceListOptions{} は無フィルタのリスト取得となる。
//
// 注意: M54 時点では invoices の Ransack _eq / _gteq 形式が実 API で有効かは未検証。
// E2E テスト（TestE2E_Invoices_M54）で確認予定。
//
// Introduced in M54 as the Phase L invoices migration target.
// Replaces the pre-M54 InvoiceSearchParams struct.
type InvoiceListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）。
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// invoices 専用
	ClientIDEq    int    // 顧客 ID 完全一致
	ProjectIDEq   int    // プロジェクト ID 完全一致
	StatusEq      string // ステータス完全一致
	ResponseGroup string // "small" / "large"
}

// InvoiceSearchParams は SearchInvoices の後方互換のために残す。
//
// Deprecated: M54 以降は InvoiceListOptions を使用すること。
// M57 で一括削除予定。
type InvoiceSearchParams struct {
	ClientID      int
	ProjectID     int
	Status        string
	UpdatedAtFrom string
}

// buildInvoicesQuery は InvoiceListOptions を Ransack スタイルのクエリ文字列に変換する。
func buildInvoicesQuery(opts InvoiceListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("client_id", opts.ClientIDEq).
		IntEq("project_id", opts.ProjectIDEq).
		StrEq("status", opts.StatusEq).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		ResponseGroup(opts.ResponseGroup).
		Encode()
}

// ListInvoices は InvoiceListOptions でフィルタした請求書一覧を返す。
// ページネーションは ListAllWithResult が内部で処理する。
//
// Pass InvoiceListOptions{} for an unfiltered list of all invoices.
func (c *Client) ListInvoices(ctx context.Context, opts InvoiceListOptions) (*ListResult[InvoiceEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/invoices", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildInvoicesQuery(opts, page, pp)
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
	items := make([]InvoiceEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x InvoiceEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListInvoices: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[InvoiceEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetInvoice は指定 ID の請求書を返す。
func (c *Client) GetInvoice(ctx context.Context, id int) (*ItemResult[InvoiceEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/invoices/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x InvoiceEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetInvoice: unmarshal: " + err.Error()}
	}
	return &ItemResult[InvoiceEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListInvoicesRaw は請求書一覧を生 JSON バイト列と最終ページのヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は ListInvoices を使うこと。
func (c *Client) ListInvoicesRaw(ctx context.Context, opts InvoiceListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/invoices", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildInvoicesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListInvoicesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetInvoiceRaw は指定 ID の請求書を生 JSON バイト列とヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は GetInvoice を使うこと。
func (c *Client) GetInvoiceRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/invoices/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
