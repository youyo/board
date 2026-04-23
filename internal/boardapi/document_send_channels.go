package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DocumentSendChannelEntity is a BOARD API document send channel entity.
// Corresponds to one element in the GET /v1/document_send_channels response.
type DocumentSendChannelEntity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Memo      string `json:"memo"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601
}

// DocumentSendChannelListOptions は GET /v1/document_send_channels のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。DocumentSendChannelListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 DocumentSendChannelSearchParams を置き換える破壊的変更。
type DocumentSendChannelListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// document_send_channels 専用（Ransack 準拠）
	NameCont string // 書類送付方法名部分一致（Ransack _cont）
}

// buildDocumentSendChannelsQuery は GET /v1/document_send_channels の Ransack スタイルクエリ文字列を組み立てる。
func buildDocumentSendChannelsQuery(opts DocumentSendChannelListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListDocumentSendChannels は与えられたオプションでフィルタした書類送付方法を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は DocumentSendChannelListOptions{} を渡す。
func (c *Client) ListDocumentSendChannels(ctx context.Context, opts DocumentSendChannelListOptions) (*ListResult[DocumentSendChannelEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/document_send_channels", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildDocumentSendChannelsQuery(opts, page, pp)
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
	items := make([]DocumentSendChannelEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x DocumentSendChannelEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListDocumentSendChannels: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[DocumentSendChannelEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetDocumentSendChannel は指定 ID の書類送付方法を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetDocumentSendChannel(ctx context.Context, id int) (*ItemResult[DocumentSendChannelEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/document_send_channels/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x DocumentSendChannelEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetDocumentSendChannel: unmarshal: " + err.Error()}
	}
	return &ItemResult[DocumentSendChannelEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListDocumentSendChannelsRaw は与えられたオプションでフィルタした書類送付方法の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListDocumentSendChannels を使うこと。
func (c *Client) ListDocumentSendChannelsRaw(ctx context.Context, opts DocumentSendChannelListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/document_send_channels", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildDocumentSendChannelsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListDocumentSendChannelsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetDocumentSendChannelRaw は指定 ID の書類送付方法の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetDocumentSendChannel を使うこと。
func (c *Client) GetDocumentSendChannelRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/document_send_channels/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
