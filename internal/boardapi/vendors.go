package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// VendorEntity は BOARD API の仕入先エンティティ。
// GET /v1/payees および GET /v1/payees/{id} のレスポンスに対応する。
// 注意: BOARD API の実パスは /v1/payees（/v1/vendors ではない）。
type VendorEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// VendorListOptions は GET /v1/payees のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。VendorListOptions{} はフィルタなしの全件取得を意味する。
//
// M55 で導入。旧 VendorSearchParams を置き換える破壊的変更。
type VendorListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// vendors 専用（Ransack 準拠）
	NameCont string // 仕入先名部分一致（Ransack _cont）
}

// buildVendorsQuery は GET /v1/payees の Ransack スタイルクエリ文字列を組み立てる。
func buildVendorsQuery(opts VendorListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListVendors は与えられたオプションでフィルタした仕入先を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は VendorListOptions{} を渡す。
func (c *Client) ListVendors(ctx context.Context, opts VendorListOptions) (*ListResult[VendorEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorsQuery(opts, page, pp)
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
	items := make([]VendorEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x VendorEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendors: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[VendorEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetVendor は指定 ID の仕入先を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetVendor(ctx context.Context, id int) (*ItemResult[VendorEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payees/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x VendorEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendor: unmarshal: " + err.Error()}
	}
	return &ItemResult[VendorEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListVendorsRaw は与えられたオプションでフィルタした仕入先の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListVendors を使うこと。
//
// 注意: BOARD API の実パスは /v1/payees（/v1/vendors ではない）。
func (c *Client) ListVendorsRaw(ctx context.Context, opts VendorListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payees", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetVendorRaw は指定 ID の仕入先の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetVendor を使うこと。
func (c *Client) GetVendorRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payees/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
