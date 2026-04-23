package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// UserEntity is a BOARD API user entity.
// Corresponds to one element in the GET /v1/users response.
type UserEntity struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	LastName     string `json:"last_name"`
	FirstName    string `json:"first_name"`
	Email        string `json:"email"`
	RoleID       int    `json:"role_id"`
	RoleName     string `json:"role_name"`
	LastSignInAt string `json:"last_sign_in_at"` // ISO 8601
	ValidFlg     int    `json:"valid_flg"`
	UpdatedAt    string `json:"updated_at"` // ISO 8601
	CreatedAt    string `json:"created_at"` // ISO 8601
}

// DisplayName returns a human-readable name.
// Prefers Name if set, otherwise combines LastName + FirstName.
func (u UserEntity) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	switch {
	case u.LastName != "" && u.FirstName != "":
		return u.LastName + " " + u.FirstName
	case u.LastName != "":
		return u.LastName
	case u.FirstName != "":
		return u.FirstName
	default:
		return ""
	}
}

// UserListOptions は GET /v1/users のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。UserListOptions{} はフィルタなしの全件取得を意味する。
//
// M56 で導入。旧 UserSearchParams を置き換える破壊的変更。
type UserListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// users 専用（Ransack 準拠）
	NameCont  string // ユーザー名部分一致（Ransack _cont）
	EmailCont string // メールアドレス部分一致（Ransack _cont）
}

// buildUsersQuery は GET /v1/users の Ransack スタイルクエリ文字列を組み立てる。
func buildUsersQuery(opts UserListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		StrCont("email", opts.EmailCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListUsers は与えられたオプションでフィルタしたユーザーを取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータは
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は UserListOptions{} を渡す。
func (c *Client) ListUsers(ctx context.Context, opts UserListOptions) (*ListResult[UserEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/users", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildUsersQuery(opts, page, pp)
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
	items := make([]UserEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x UserEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListUsers: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[UserEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetUser は指定 ID のユーザーを取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetUser(ctx context.Context, id int) (*ItemResult[UserEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x UserEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetUser: unmarshal: " + err.Error()}
	}
	return &ItemResult[UserEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListUsersRaw は与えられたオプションでフィルタしたユーザーの生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListUsers を使うこと。
func (c *Client) ListUsersRaw(ctx context.Context, opts UserListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/users", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildUsersQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListUsersRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetUserRaw は指定 ID のユーザーの生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetUser を使うこと。
func (c *Client) GetUserRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
