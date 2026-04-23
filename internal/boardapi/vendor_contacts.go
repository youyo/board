package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// VendorContactEntity は BOARD API の仕入先担当者エンティティ。
// GET /v1/payee_contacts のレスポンス 1 要素に対応する。
// ContactEntity（M40 再設計）と同型のパターンを vendor 側に適用し M42 で全面再設計。
//
// 注意: nested オブジェクトのキー（"vendor"）は未確認（アカウントのデータが 0 件のため）。
// ContactEntity が "client" ネストを使うことを確認済みなので "vendor" と推定する。
// データ投入後の smoke テスト（TestE2E_VendorContacts_*）で Pending Re-verification。
type VendorContactEntity struct {
	ID             int        `json:"id"`
	Vendor         *VendorRef `json:"vendor"` // nested 構造: {id, name, name_disp, custom_no}（未確認）
	LastName       string     `json:"last_name"`
	FirstName      string     `json:"first_name"`
	HonorificTitle string     `json:"honorific_title"`
	Title          *string    `json:"title"`      // null 可
	Department     *string    `json:"department"` // null 可
	Email          *string    `json:"email"`      // null 可
	Note           *string    `json:"note"`       // null 可
	ArchiveFlg     int        `json:"archive_flg"`
	CreatedAt      string     `json:"created_at"` // ISO 8601
	UpdatedAt      string     `json:"updated_at"` // ISO 8601
}

// VendorID は nested Vendor.ID を返す accessor（後方互換ブリッジ）。
// Vendor が nil の場合 0 を返す。
func (e VendorContactEntity) VendorID() int {
	if e.Vendor == nil {
		return 0
	}
	return e.Vendor.ID
}

// DisplayName は人名を返す。LastName + FirstName を結合する。
// Name フィールドは M42 再設計で廃止（ContactEntity と同様）。
func (e VendorContactEntity) DisplayName() string {
	switch {
	case e.LastName != "" && e.FirstName != "":
		return e.LastName + " " + e.FirstName
	case e.LastName != "":
		return e.LastName
	case e.FirstName != "":
		return e.FirstName
	default:
		return ""
	}
}

// VendorContactListOptions は GET /v1/payee_contacts のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。VendorContactListOptions{} はフィルタなしの全件取得を意味する。
//
// M55 で導入。旧 VendorContactSearchParams を置き換える破壊的変更。
//
// 注意: PayeeIDEq の Ransack パラメータ名は payee_id_eq（BOARD API の URL が /v1/payees である
// ため Rails モデル名は Payee と推定）。E2E テストで実際の挙動を確認すること。
type VendorContactListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// vendor_contacts 専用（Ransack 準拠）
	PayeeIDEq int    // 仕入先 ID 完全一致（Ransack payee_id_eq）
	NameCont  string // 担当者名部分一致（Ransack _cont）
	EmailCont string // メールアドレス部分一致
}

// buildVendorContactsQuery は GET /v1/payee_contacts の Ransack スタイルクエリ文字列を組み立てる。
func buildVendorContactsQuery(opts VendorContactListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("payee_id", opts.PayeeIDEq).
		StrCont("name", opts.NameCont).
		StrCont("email", opts.EmailCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListVendorContacts は与えられたオプションでフィルタした仕入先担当者を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は VendorContactListOptions{} を渡す。
func (c *Client) ListVendorContacts(ctx context.Context, opts VendorContactListOptions) (*ListResult[VendorContactEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_contacts", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorContactsQuery(opts, page, pp)
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
	items := make([]VendorContactEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x VendorContactEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorContacts: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[VendorContactEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetVendorContact は指定 ID の仕入先担当者を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetVendorContact(ctx context.Context, id int) (*ItemResult[VendorContactEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x VendorContactEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendorContact: unmarshal: " + err.Error()}
	}
	return &ItemResult[VendorContactEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListVendorContactsRaw は与えられたオプションでフィルタした仕入先担当者の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListVendorContacts を使うこと。
//
// 注意: BOARD API の実パスは /v1/payee_contacts（/v1/vendor_contacts ではない）。
func (c *Client) ListVendorContactsRaw(ctx context.Context, opts VendorContactListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_contacts", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorContactsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorContactsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetVendorContactRaw は指定 ID の仕入先担当者の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetVendorContact を使うこと。
func (c *Client) GetVendorContactRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_contacts/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
