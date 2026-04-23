package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PaymentEntity は BOARD API の支払エンティティ。
// GET /v1/expenditure_payments および GET /v1/expenditure_payments/{id} のレスポンスに対応する。
// 注意: 実 API パスは /v1/expenditure_payments（Go 名: payments）。
type PaymentEntity struct {
	ID              int     `json:"id"`
	VendorID        int     `json:"vendor_id"`
	PurchaseOrderID int     `json:"purchase_order_id"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status"`
	PaymentDate     string  `json:"payment_date"` // ISO 8601 date
	Memo            string  `json:"memo"`
	UpdatedAt       string  `json:"updated_at"` // ISO 8601
	CreatedAt       string  `json:"created_at"` // ISO 8601
}

// PaymentListOptions は GET /v1/expenditure_payments のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信されないため、PaymentListOptions{} は無フィルタのリスト取得となる。
//
// 注意: M54 時点では expenditure_payments の Ransack _eq / _gteq 形式が実 API で有効かは未検証。
// E2E テスト（TestE2E_Payments_M54）で確認予定。
//
// Introduced in M54 as the Phase L payments migration target.
// Replaces the pre-M54 PaymentSearchParams struct.
type PaymentListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）。
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// payments 専用
	VendorIDEq        int    // 取引先 ID 完全一致
	PurchaseOrderIDEq int    // 発注書 ID 完全一致
	StatusEq          string // ステータス完全一致
	ResponseGroup     string // "small" / "large"
}

// PaymentSearchParams は SearchPayments の後方互換のために残す。
//
// Deprecated: M54 以降は PaymentListOptions を使用すること。
// M57 で一括削除予定。
type PaymentSearchParams struct {
	VendorID        int
	PurchaseOrderID int
	Status          string
	UpdatedAtFrom   string
}

// buildPaymentsQuery は PaymentListOptions を Ransack スタイルのクエリ文字列に変換する。
func buildPaymentsQuery(opts PaymentListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("vendor_id", opts.VendorIDEq).
		IntEq("purchase_order_id", opts.PurchaseOrderIDEq).
		StrEq("status", opts.StatusEq).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		ResponseGroup(opts.ResponseGroup).
		Encode()
}

// ListPayments は PaymentListOptions でフィルタした支払一覧を返す。
// ページネーションは ListAllWithResult が内部で処理する。
//
// 注意: 実 API パスは /v1/expenditure_payments（Go 名: payments）。
//
// Pass PaymentListOptions{} for an unfiltered list of all payments.
func (c *Client) ListPayments(ctx context.Context, opts PaymentListOptions) (*ListResult[PaymentEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_payments", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPaymentsQuery(opts, page, pp)
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
	items := make([]PaymentEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x PaymentEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPayments: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[PaymentEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetPayment は指定 ID の支払を返す。
// 注意: 実 API パスは /v1/expenditure_payments/{id}。
func (c *Client) GetPayment(ctx context.Context, id int) (*ItemResult[PaymentEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_payments/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x PaymentEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPayment: unmarshal: " + err.Error()}
	}
	return &ItemResult[PaymentEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListPaymentsRaw は支払一覧を生 JSON バイト列と最終ページのヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は ListPayments を使うこと。
// 注意: 実 API パスは /v1/expenditure_payments。
func (c *Client) ListPaymentsRaw(ctx context.Context, opts PaymentListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_payments", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPaymentsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetPaymentRaw は指定 ID の支払を生 JSON バイト列とヘッダーで返す。
// E2E strict field diff 専用。通常の呼び出し元は GetPayment を使うこと。
// 注意: 実 API パスは /v1/expenditure_payments/{id}。
func (c *Client) GetPaymentRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_payments/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
