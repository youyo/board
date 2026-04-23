package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ContactEntity は BOARD API の担当者エンティティ。
// GET /v1/contacts および GET /v1/contacts/{id} のレスポンスに対応する。
// 実 API レスポンスに基づき M40 で全面再設計。
type ContactEntity struct {
	ID             int        `json:"id"`
	Client         *ClientRef `json:"client"`
	LastName       string     `json:"last_name"`
	FirstName      string     `json:"first_name"`
	HonorificTitle string     `json:"honorific_title"`
	Title          *string    `json:"title"`
	Department     *string    `json:"department"`
	Email          *string    `json:"email"`
	Note           *string    `json:"note"`
	ArchiveFlg     int        `json:"archive_flg"`
	CreatedAt      string     `json:"created_at"` // ISO 8601
	UpdatedAt      string     `json:"updated_at"` // ISO 8601
}

// ClientID は親顧客の ID を返す。
// nested Client が nil の場合 0 を返す。
func (c ContactEntity) ClientID() int {
	if c.Client == nil {
		return 0
	}
	return c.Client.ID
}

// DisplayName は LastName と FirstName を組み合わせた表示名を返す。
func (c ContactEntity) DisplayName() string {
	switch {
	case c.LastName != "" && c.FirstName != "":
		return c.LastName + " " + c.FirstName
	case c.LastName != "":
		return c.LastName
	case c.FirstName != "":
		return c.FirstName
	default:
		return ""
	}
}

// ContactListOptions は GET /v1/contacts のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。ContactListOptions{} はフィルタなしの全件取得を意味する。
//
// M52 で導入。旧 ContactSearchParams を置き換える破壊的変更。
type ContactListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// contacts 専用（Ransack 準拠）
	ClientIDEq int    // 顧客 ID 完全一致
	NameCont   string // 担当者名部分一致（Ransack _cont）
	EmailCont  string // メールアドレス部分一致
}

// buildContactsQuery は GET /v1/contacts の Ransack スタイルクエリ文字列を組み立てる。
func buildContactsQuery(opts ContactListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("client_id", opts.ClientIDEq).
		StrCont("name", opts.NameCont).
		StrCont("email", opts.EmailCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListContacts は与えられたオプションでフィルタした担当者を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は ContactListOptions{} を渡す。
func (c *Client) ListContacts(ctx context.Context, opts ContactListOptions) (*ListResult[ContactEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/contacts", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildContactsQuery(opts, page, pp)
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
	items := make([]ContactEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ContactEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListContacts: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ContactEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetContact は指定 ID の担当者を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetContact(ctx context.Context, id int) (*ItemResult[ContactEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ContactEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetContact: unmarshal: " + err.Error()}
	}
	return &ItemResult[ContactEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListContactsRaw は与えられたオプションでフィルタした担当者の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListContacts を使うこと。
func (c *Client) ListContactsRaw(ctx context.Context, opts ContactListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/contacts", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildContactsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetContactRaw は指定 ID の担当者の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetContact を使うこと。
func (c *Client) GetContactRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/contacts/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
