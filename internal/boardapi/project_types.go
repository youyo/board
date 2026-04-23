package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ProjectTypeEntity is a BOARD API project type entity.
// Corresponds to one element in the GET /v1/project_types response.
type ProjectTypeEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// ProjectTypeListOptions は GET /v1/project_types のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。ProjectTypeListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 ProjectTypeSearchParams を置き換える破壊的変更。
type ProjectTypeListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// project_types 専用（Ransack 準拠）
	NameCont string // 案件種別名部分一致（Ransack _cont）
}

// buildProjectTypesQuery は GET /v1/project_types の Ransack スタイルクエリ文字列を組み立てる。
func buildProjectTypesQuery(opts ProjectTypeListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListProjectTypes は与えられたオプションでフィルタした案件種別を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は ProjectTypeListOptions{} を渡す。
func (c *Client) ListProjectTypes(ctx context.Context, opts ProjectTypeListOptions) (*ListResult[ProjectTypeEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildProjectTypesQuery(opts, page, pp)
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
	items := make([]ProjectTypeEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ProjectTypeEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectTypes: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ProjectTypeEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetProjectType は指定 ID の案件種別を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetProjectType(ctx context.Context, id int) (*ItemResult[ProjectTypeEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_types/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ProjectTypeEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectType: unmarshal: " + err.Error()}
	}
	return &ItemResult[ProjectTypeEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListProjectTypesRaw は与えられたオプションでフィルタした案件種別の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListProjectTypes を使うこと。
func (c *Client) ListProjectTypesRaw(ctx context.Context, opts ProjectTypeListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_types", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildProjectTypesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectTypesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetProjectTypeRaw は指定 ID の案件種別の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetProjectType を使うこと。
func (c *Client) GetProjectTypeRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_types/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
