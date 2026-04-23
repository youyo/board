package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ClientBranchEntity は BOARD API の顧客支社エンティティ。
// GET /v1/client_branches のレスポンス 1 要素に対応する。
// 実 API レスポンス（tmp/e2e-artifacts/client_branches_*.json）に基づき M39 で全面再設計。
type ClientBranchEntity struct {
	ID         int        `json:"id"`
	Client     *ClientRef `json:"client"` // nested 構造: {id, name, name_disp, custom_no}
	Name       string     `json:"name"`
	Zip        string     `json:"zip"`
	Pref       string     `json:"pref"`
	Address1   string     `json:"address1"`
	Address2   string     `json:"address2"`
	Tel        *string    `json:"tel"` // null 可
	Fax        *string    `json:"fax"` // null 可
	ArchiveFlg int        `json:"archive_flg"`
	CreatedAt  string     `json:"created_at"` // ISO 8601
	UpdatedAt  string     `json:"updated_at"` // ISO 8601
}

// ClientID は nested Client.ID を返す accessor（後方互換ブリッジ）。
// Client が nil の場合 0 を返す。
func (e ClientBranchEntity) ClientID() int {
	if e.Client == nil {
		return 0
	}
	return e.Client.ID
}

// ClientBranchListOptions は GET /v1/client_branches のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。ClientBranchListOptions{} はフィルタなしの全件取得を意味する。
//
// M52 で導入。旧 ClientBranchSearchParams を置き換える破壊的変更。
type ClientBranchListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// client_branches 専用（Ransack 準拠）
	ClientIDEq int    // 顧客 ID 完全一致
	NameCont   string // 支社名部分一致（Ransack _cont）
}

// buildClientBranchesQuery は GET /v1/client_branches の Ransack スタイルクエリ文字列を組み立てる。
func buildClientBranchesQuery(opts ClientBranchListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("client_id", opts.ClientIDEq).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListClientBranches は与えられたオプションでフィルタした顧客支社を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は ClientBranchListOptions{} を渡す。
func (c *Client) ListClientBranches(ctx context.Context, opts ClientBranchListOptions) (*ListResult[ClientBranchEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildClientBranchesQuery(opts, page, pp)
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
	items := make([]ClientBranchEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ClientBranchEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListClientBranches: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ClientBranchEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetClientBranch は指定 ID の顧客支社を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetClientBranch(ctx context.Context, id int) (*ItemResult[ClientBranchEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/client_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ClientBranchEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetClientBranch: unmarshal: " + err.Error()}
	}
	return &ItemResult[ClientBranchEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListClientBranchesRaw は与えられたオプションでフィルタした顧客支社の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListClientBranches を使うこと。
func (c *Client) ListClientBranchesRaw(ctx context.Context, opts ClientBranchListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/client_branches", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildClientBranchesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListClientBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetClientBranchRaw は指定 ID の顧客支社の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetClientBranch を使うこと。
func (c *Client) GetClientBranchRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/client_branches/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
