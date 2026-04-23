package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// VendorBranchEntity は BOARD API の仕入先支社エンティティ。
// GET /v1/payee_branches のレスポンス 1 要素に対応する。
// 実 API 一貫性（ClientBranchEntity / M39 再設計）に基づき M41 で全面再設計。
//
// 注意: nested オブジェクトのキー（"vendor"）は未確認（アカウントのデータが 0 件のため）。
// ClientBranch が "client" ネストを使うことを確認済みなので "vendor" と推定する。
// データ投入後の smoke テスト（TestE2E_VendorBranches_*）で Pending Re-verification。
type VendorBranchEntity struct {
	ID         int        `json:"id"`
	Vendor     *VendorRef `json:"vendor"` // nested 構造: {id, name, name_disp, custom_no}（未確認）
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

// VendorID は nested Vendor.ID を返す accessor（後方互換ブリッジ）。
// Vendor が nil の場合 0 を返す。
func (e VendorBranchEntity) VendorID() int {
	if e.Vendor == nil {
		return 0
	}
	return e.Vendor.ID
}

// VendorBranchListOptions は GET /v1/payee_branches のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。VendorBranchListOptions{} はフィルタなしの全件取得を意味する。
//
// M55 で導入。旧 VendorBranchSearchParams を置き換える破壊的変更。
//
// 注意: PayeeIDEq の Ransack パラメータ名は payee_id_eq（BOARD API の URL が /v1/payees である
// ため Rails モデル名は Payee と推定）。E2E テストで実際の挙動を確認すること。
type VendorBranchListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// vendor_branches 専用（Ransack 準拠）
	PayeeIDEq int    // 仕入先 ID 完全一致（Ransack payee_id_eq）
	NameCont  string // 支社名部分一致（Ransack _cont）
}

// buildVendorBranchesQuery は GET /v1/payee_branches の Ransack スタイルクエリ文字列を組み立てる。
func buildVendorBranchesQuery(opts VendorBranchListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("payee_id", opts.PayeeIDEq).
		StrCont("name", opts.NameCont).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListVendorBranches は与えられたオプションでフィルタした仕入先支社を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は VendorBranchListOptions{} を渡す。
func (c *Client) ListVendorBranches(ctx context.Context, opts VendorBranchListOptions) (*ListResult[VendorBranchEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorBranchesQuery(opts, page, pp)
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
	items := make([]VendorBranchEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x VendorBranchEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorBranches: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[VendorBranchEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetVendorBranch は指定 ID の仕入先支社を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetVendorBranch(ctx context.Context, id int) (*ItemResult[VendorBranchEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_branches/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x VendorBranchEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetVendorBranch: unmarshal: " + err.Error()}
	}
	return &ItemResult[VendorBranchEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListVendorBranchesRaw は与えられたオプションでフィルタした仕入先支社の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListVendorBranches を使うこと。
//
// 注意: BOARD API の実パスは /v1/payee_branches（/v1/vendor_branches ではない）。
func (c *Client) ListVendorBranchesRaw(ctx context.Context, opts VendorBranchListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/payee_branches", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildVendorBranchesQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListVendorBranchesRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetVendorBranchRaw は指定 ID の仕入先支社の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetVendorBranch を使うこと。
func (c *Client) GetVendorBranchRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/payee_branches/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
