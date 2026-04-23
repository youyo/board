package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PurchaseTypeEntity is a BOARD API purchase type entity.
// Corresponds to one element in the GET /v1/expenditure_types response.
// 注意: BOARD API の実パスは /v1/expenditure_types（/v1/purchase_types ではない）。
type PurchaseTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// PurchaseTypeListOptions は GET /v1/expenditure_types のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。PurchaseTypeListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 PurchaseTypeSearchParams を置き換える破壊的変更。
type PurchaseTypeListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// purchase_types 専用（Ransack 準拠）
	NameCont string // 仕入種別名部分一致（Ransack _cont）
}

// buildPurchaseTypesQuery は GET /v1/expenditure_types の Ransack スタイルクエリ文字列を組み立てる。
func buildPurchaseTypesQuery(opts PurchaseTypeListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListPurchaseTypes は与えられたオプションでフィルタした仕入種別を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// 注意: BOARD API の実パスは /v1/expenditure_types（/v1/purchase_types ではない）。
// フィルタなしの全件取得は PurchaseTypeListOptions{} を渡す。
func (c *Client) ListPurchaseTypes(ctx context.Context, opts PurchaseTypeListOptions) (*ListResult[PurchaseTypeEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPurchaseTypesQuery(opts, page, pp)
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
	items := make([]PurchaseTypeEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x PurchaseTypeEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseTypes: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[PurchaseTypeEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetPurchaseType は指定 ID の仕入種別を取得する。
// 注意: BOARD API の実パスは /v1/expenditure_types/{id}。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetPurchaseType(ctx context.Context, id int) (*ItemResult[PurchaseTypeEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x PurchaseTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetPurchaseType: unmarshal: " + err.Error()}
	}
	return &ItemResult[PurchaseTypeEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListPurchaseTypesRaw は与えられたオプションでフィルタした仕入種別の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListPurchaseTypes を使うこと。
func (c *Client) ListPurchaseTypesRaw(ctx context.Context, opts PurchaseTypeListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/expenditure_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildPurchaseTypesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListPurchaseTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetPurchaseTypeRaw は指定 ID の仕入種別の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetPurchaseType を使うこと。
func (c *Client) GetPurchaseTypeRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/expenditure_types/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
