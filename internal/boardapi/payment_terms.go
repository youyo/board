package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PaymentTermEntity is a BOARD API payment term entity.
// Corresponds to one element in the GET /v1/payment_terms response.
type PaymentTermEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PaymentTermListOptions は GET /v1/payment_terms のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。PaymentTermListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 PaymentTermSearchParams を置き換える破壊的変更。
type PaymentTermListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// payment_terms 専用（Ransack 準拠）
	NameCont string // 支払条件名部分一致（Ransack _cont）
}

// buildPaymentTermsQuery は GET /v1/payment_terms の Ransack スタイルクエリ文字列を組み立てる。
func buildPaymentTermsQuery(opts PaymentTermListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListPaymentTerms は与えられたオプションでフィルタした支払条件を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は PaymentTermListOptions{} を渡す。
func (c *Client) ListPaymentTerms(ctx context.Context, opts PaymentTermListOptions) (*ListResult[PaymentTermEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPaymentTermsQuery(opts, page, pp)
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
	items := make([]PaymentTermEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x PaymentTermEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentTerms: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[PaymentTermEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetPaymentTerm は指定 ID の支払条件を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetPaymentTerm(ctx context.Context, id int) (*ItemResult[PaymentTermEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payment_terms/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x PaymentTermEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPaymentTerm: unmarshal: " + err.Error()}
	}
	return &ItemResult[PaymentTermEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListPaymentTermsRaw は与えられたオプションでフィルタした支払条件の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListPaymentTerms を使うこと。
func (c *Client) ListPaymentTermsRaw(ctx context.Context, opts PaymentTermListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payment_terms", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPaymentTermsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListPaymentTermsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetPaymentTermRaw は指定 ID の支払条件の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetPaymentTerm を使うこと。
func (c *Client) GetPaymentTermRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payment_terms/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
