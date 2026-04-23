package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// AccountingTypeEntity is a BOARD API accounting type entity.
// Corresponds to one element in the GET /v1/accounting_types response.
type AccountingTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// AccountingTypeListOptions は GET /v1/accounting_types のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。AccountingTypeListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 AccountingTypeSearchParams を置き換える破壊的変更。
type AccountingTypeListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// accounting_types 専用（Ransack 準拠）
	NameCont string // 経理種別名部分一致（Ransack _cont）
}

// buildAccountingTypesQuery は GET /v1/accounting_types の Ransack スタイルクエリ文字列を組み立てる。
func buildAccountingTypesQuery(opts AccountingTypeListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListAccountingTypes は与えられたオプションでフィルタした経理種別を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は AccountingTypeListOptions{} を渡す。
func (c *Client) ListAccountingTypes(ctx context.Context, opts AccountingTypeListOptions) (*ListResult[AccountingTypeEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildAccountingTypesQuery(opts, page, pp)
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
	items := make([]AccountingTypeEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x AccountingTypeEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListAccountingTypes: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[AccountingTypeEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetAccountingType は指定 ID の経理種別を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetAccountingType(ctx context.Context, id int) (*ItemResult[AccountingTypeEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/accounting_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x AccountingTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetAccountingType: unmarshal: " + err.Error()}
	}
	return &ItemResult[AccountingTypeEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListAccountingTypesRaw は与えられたオプションでフィルタした経理種別の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListAccountingTypes を使うこと。
func (c *Client) ListAccountingTypesRaw(ctx context.Context, opts AccountingTypeListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/accounting_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildAccountingTypesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListAccountingTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetAccountingTypeRaw は指定 ID の経理種別の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetAccountingType を使うこと。
func (c *Client) GetAccountingTypeRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/accounting_types/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
