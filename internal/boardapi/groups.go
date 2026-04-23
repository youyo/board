package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GroupEntity is a BOARD API group entity.
// Corresponds to one element in the GET /v1/groups response.
type GroupEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// GroupListOptions は GET /v1/groups のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。GroupListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 GroupSearchParams を置き換える破壊的変更。
type GroupListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// groups 専用（Ransack 準拠）
	NameCont string // グループ名部分一致（Ransack _cont）
}

// buildGroupsQuery は GET /v1/groups の Ransack スタイルクエリ文字列を組み立てる。
func buildGroupsQuery(opts GroupListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListGroups は与えられたオプションでフィルタしたグループを取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は GroupListOptions{} を渡す。
func (c *Client) ListGroups(ctx context.Context, opts GroupListOptions) (*ListResult[GroupEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/groups", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildGroupsQuery(opts, page, pp)
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
	items := make([]GroupEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x GroupEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListGroups: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[GroupEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetGroup は指定 ID のグループを取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetGroup(ctx context.Context, id int) (*ItemResult[GroupEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/groups/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x GroupEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetGroup: unmarshal: " + err.Error()}
	}
	return &ItemResult[GroupEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListGroupsRaw は与えられたオプションでフィルタしたグループの生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListGroups を使うこと。
func (c *Client) ListGroupsRaw(ctx context.Context, opts GroupListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/groups", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildGroupsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListGroupsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetGroupRaw は指定 ID のグループの生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetGroup を使うこと。
func (c *Client) GetGroupRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/groups/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
