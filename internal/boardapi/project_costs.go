package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ProjectCostEntity は BOARD API の ProjectCost エンティティ。
// GET /v1/project_costs および GET /v1/project_costs/{id} の実レスポンスに
// 完全一致する 8 フィールド定義。
//
// 実データ根拠: tmp/e2e-artifacts/project_costs_33291004.json
// 概念: プロジェクト原価台帳の行（個別支払い記録）
type ProjectCostEntity struct {
	ID          int     `json:"id"`
	ProjectID   int     `json:"project_id"`
	Description string  `json:"description"`  // 支払い内容の説明
	Cost        int     `json:"cost"`         // 金額（整数）
	InvoiceDate *string `json:"invoice_date"` // 請求日（ISO date）、null 可
	PaymentDate *string `json:"payment_date"` // 支払日（ISO date）、null 可
	UpdatedAt   string  `json:"updated_at"`   // ISO 8601
	CreatedAt   string  `json:"created_at"`   // ISO 8601
}

// ProjectCostListOptions は GET /v1/project_costs のクエリパラメータ（Ransack スタイル）。
// ゼロ値は API に送信しない。ProjectCostListOptions{} はフィルタなしの全件取得を意味する。
//
// M52 で導入。旧 ProjectCostSearchParams を置き換える破壊的変更。
type ProjectCostListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0

	// project_costs 専用（Ransack 準拠）
	ProjectIDEq int // プロジェクト ID 完全一致
}

// buildProjectCostsQuery は GET /v1/project_costs の Ransack スタイルクエリ文字列を組み立てる。
func buildProjectCostsQuery(opts ProjectCostListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		IntEq("project_id", opts.ProjectIDEq).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Encode()
}

// ListProjectCosts は与えられたオプションでフィルタしたプロジェクト原価を取得する。
// ページネーションは ListAllWithResult が内部で処理する。メタデータ（件数・レート制限・ETag）は
// 返り値の *ListResult 経由で参照できる。
//
// フィルタなしの全件取得は ProjectCostListOptions{} を渡す。
func (c *Client) ListProjectCosts(ctx context.Context, opts ProjectCostListOptions) (*ListResult[ProjectCostEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildProjectCostsQuery(opts, page, pp)
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
	items := make([]ProjectCostEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ProjectCostEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectCosts: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ProjectCostEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetProjectCost は指定 ID のプロジェクト原価を取得する。
// レスポンスメタデータ（ETag・レート制限・Last-Modified）は *ItemResult 経由で参照できる。
func (c *Client) GetProjectCost(ctx context.Context, id int) (*ItemResult[ProjectCostEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_costs/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ProjectCostEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectCost: unmarshal: " + err.Error()}
	}
	return &ItemResult[ProjectCostEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListProjectCostsRaw は与えられたオプションでフィルタしたプロジェクト原価の生 JSON 配列と
// 最終ページのレスポンスヘッダーを返す。バイト列は BOARD API が返したものをそのまま保持するため、
// E2E の strict field diff に使用できる。通常の呼び出しには ListProjectCosts を使うこと。
func (c *Client) ListProjectCostsRaw(ctx context.Context, opts ProjectCostListOptions) ([]byte, http.Header, error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/project_costs", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildProjectCostsQuery(opts, page, pp)
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
		return nil, nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectCostsRaw: marshal aggregate: " + err.Error()}
	}
	return out, raw.Headers, nil
}

// GetProjectCostRaw は指定 ID のプロジェクト原価の生 HTTP レスポンスボディとヘッダーを返す。
// E2E の strict field diff に使用できる。通常の呼び出しには GetProjectCost を使うこと。
func (c *Client) GetProjectCostRaw(ctx context.Context, id int) ([]byte, http.Header, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/project_costs/%d", id), nil)
	if err != nil {
		return nil, nil, err
	}
	return c.DoWithRetryFull(req)
}
