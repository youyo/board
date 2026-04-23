package boardapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ProjectEntity は BOARD API の Project エンティティ。
// GET /v1/projects および GET /v1/projects/{id}（response_group 6 バリアント含む）の
// 実レスポンスに完全一致する 72 トップレベルフィールド定義。
//
// 3 段階 response モデル:
//   - List/Search 共通 (25 キー): ID/Name/ProjectNo/ManagementNo/Client/Contact/User/
//     Total/Tax/EstimateDate/InvoiceDates/OrderStatus*/DeliveryStatus*/ProjectType{,2,3}*/
//     GroupID/GroupName/CreatedAt/UpdatedAt
//   - プレーン Get (72 キー): List 共通 + Get 限定 47 フィールド
//   - rg=all (72 キー + document arrays): プレーン Get + Estimate/Order/Deliveries/Invoices/Receipts/ProjectCosts
type ProjectEntity struct {
	// ===== 共通 4 フィールド（List/Search/Get 全てで出現） =====
	ID        int    `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"` // ISO 8601
	CreatedAt string `json:"created_at"` // ISO 8601

	// ===== List/Search 共通 21 フィールド =====
	ProjectNo    *int    `json:"project_no"`    // 4106 等。null 可
	ManagementNo *string `json:"management_no"` // dump は null。運用で使用される場合あり
	// Client は nested {id, name, name_disp, custom_no}。M39 既存 ClientRef 再利用。
	Client  *ClientRef  `json:"client"`
	Contact *ContactRef `json:"contact"` // Risk-3: dump 全件 null、構造未確認
	User    *UserRef    `json:"user"`    // {id, last_name, first_name}

	Total *string `json:"total"` // "90000.0" — BOARD は金額を string で返す
	Tax   *string `json:"tax"`   // "9000.0"

	EstimateDate *string  `json:"estimate_date"` // ISO date
	InvoiceDates []string `json:"invoice_dates"` // 空配列あり

	OrderStatus        int    `json:"order_status"`         // 1/2/3...
	OrderStatusName    string `json:"order_status_name"`    // 「見積中(中)」等
	DeliveryStatus     int    `json:"delivery_status"`      // 1/2/3...
	DeliveryStatusName string `json:"delivery_status_name"` // 「未着手」等

	ProjectTypeID    *int    `json:"project_type_id"`   // null 可
	ProjectTypeName  *string `json:"project_type_name"` // null 可
	ProjectType2ID   *int    `json:"project_type2_id"`
	ProjectType2Name *string `json:"project_type2_name"`
	ProjectType3ID   *int    `json:"project_type3_id"`
	ProjectType3Name *string `json:"project_type3_name"`

	GroupID   *int    `json:"group_id"`   // null 可（dump null）
	GroupName *string `json:"group_name"` // null 可

	// ===== Get 限定 47 フィールド（List/Search では出現しない） =====

	// nested 型 3 件（dump は null 観測のみ、構造未確認）
	ClientBranch  *ClientBranchRef  `json:"client_branch"`  // Risk-3
	CompanyBranch *CompanyBranchRef `json:"company_branch"` // Risk-3
	Hubspot       *HubspotRef       `json:"hubspot"`        // {hubspot_deal_id: null}

	// 表示区分（disp_kbn）系 8 フィールド
	ClientNameDispKbn             int    `json:"client_name_disp_kbn"`
	ClientNameDispKbnName         string `json:"client_name_disp_kbn_name"`
	ClientNameForPostDispKbn      int    `json:"client_name_for_post_disp_kbn"`
	ClientNameForPostDispKbnName  string `json:"client_name_for_post_disp_kbn_name"`
	CompanyNameDispKbn            int    `json:"company_name_disp_kbn"`
	CompanyNameDispKbnName        string `json:"company_name_disp_kbn_name"`
	CompanyNameForPostDispKbn     int    `json:"company_name_for_post_disp_kbn"`
	CompanyNameForPostDispKbnName string `json:"company_name_for_post_disp_kbn_name"`

	// 金額・集計系 4 フィールド
	// 注意: BOARD API は cost_total/cost_tax が int、invoice_total/invoice_tax が string という型混在
	CostTotal    int     `json:"cost_total"`    // 数値（0 観測）
	CostTax      int     `json:"cost_tax"`      // 数値
	InvoiceTotal *string `json:"invoice_total"` // "270000.0"
	InvoiceTax   *string `json:"invoice_tax"`   // "27000.0"

	// 日付系 3 フィールド
	DeliveryDate     *string `json:"delivery_date"`      // トップレベル納期
	DeliveryDateText *string `json:"delivery_date_text"` // 自由記述納期
	OrderedDate      *string `json:"ordered_date"`       // 受注日

	// 支払条件・決済系 4 フィールド
	PaymentTermID        *int    `json:"payment_term_id"`
	PaymentTermName      *string `json:"payment_term_name"`
	PaymentMethodKbn     int     `json:"payment_method_kbn"`      // 1=銀行振込 等
	PaymentMethodKbnName string  `json:"payment_method_kbn_name"` // 「銀行振込」等

	// 請求タイミング・契約管理系 11 フィールド
	InvoiceTimingKbn            int     `json:"invoice_timing_kbn"`      // 2=定期請求
	InvoiceTimingKbnName        string  `json:"invoice_timing_kbn_name"` // 「定期請求」等
	ContractStartDate           *string `json:"contract_start_date"`     // 旧 StartDate の代替
	ContractEndDate             *string `json:"contract_end_date"`       // 旧 EndDate の代替
	PeriodicalInvoiceInterval   *int    `json:"periodical_invoice_interval"`
	PeriodicalInvoicePaymentKbn *int    `json:"periodical_invoice_payment_kbn"`
	ContractEndAlertFlg         int     `json:"contract_end_alert_flg"` // 0/1
	AutoRenewalFlg              int     `json:"auto_renewal_flg"`       // 0/1
	AutoRenewalPeriodMonth      *int    `json:"auto_renewal_period_month"`
	MonthlyInvoicePaymentKbn    *int    `json:"monthly_invoice_payment_kbn"`
	DeliveryDocumentKbn         int     `json:"delivery_document_kbn"`

	// 会計区分 6 フィールド
	AccountingTypeID    *int    `json:"accounting_type_id"`   // null 可
	AccountingTypeName  *string `json:"accounting_type_name"` // null 可
	AccountingType2ID   *int    `json:"accounting_type2_id"`
	AccountingType2Name *string `json:"accounting_type2_name"`
	AccountingType3ID   *int    `json:"accounting_type3_id"`
	AccountingType3Name *string `json:"accounting_type3_name"`

	// その他 8 フィールド
	Tags                []string `json:"tags"`          // 空配列あり
	InHouseMemo         *string  `json:"in_house_memo"` // 旧 Memo の代替
	To                  *string  `json:"to"`            // 送付先 To
	CC                  *string  `json:"cc"`            // 送付先 CC
	LockFlg             int      `json:"lock_flg"`      // 0/1
	ArchiveFlg          int      `json:"archive_flg"`   // 0/1
	DocumentSettingID   *int     `json:"document_setting_id"`
	DocumentSettingName *string  `json:"document_setting_name"`

	// ===== response_group フィールド（rg 指定時のみ出現） =====
	// estimate と order は API が単一オブジェクトで返す（単数形キー）。
	Estimate *DocumentSummary `json:"estimate,omitempty"`
	Order    *DocumentSummary `json:"order,omitempty"`
	// delivery / invoice / receipt は API が複数形配列で返す。
	Deliveries []DocumentSummary `json:"deliveries,omitempty"`
	Invoices   []DocumentSummary `json:"invoices,omitempty"`
	Receipts   []DocumentSummary `json:"receipts,omitempty"`

	// ===== response_group フィールド（rg=all 時のみ出現） =====
	// ProjectCosts はプロジェクト原価台帳の行（個別支払い記録リスト）。
	ProjectCosts []ProjectCostEntity `json:"project_costs,omitempty"`
}

// ProjectListOptions は BOARD API GET /v1/projects のクエリパラメータ（Ransack スタイル）。
// ゼロ値フィールドは API リクエストに送信されない。
// ゼロ値 ProjectListOptions{} はフィルタなし（全件取得）。
//
// M51 で ProjectSearchParams に代わり導入。Phase L clients パイロット（M50）で
// 確立されたパターンを projects に適用したもの。
type ProjectListOptions struct {
	// 共通ページネーション（通常は ListAllWithResult が page を上書きする）
	Page    int
	PerPage int

	// 全 List 共通
	UpdatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	UpdatedAtLteq     string
	CreatedAtGteq     string // "YYYY-MM-DD HH:MM:SS"
	CreatedAtLteq     string
	IncludeArchiveFlg *bool // nil=送らない, true=1, false=0
	IncludeLostFlg    *bool // nil=送らない, true=1, false=0（失注案件含む）

	// projects 専用（Ransack 準拠）
	NameCont         string // プロジェクト名部分一致
	ClientIDEq       int    // 顧客 ID 完全一致
	ClientNameCont   string // 顧客名部分一致
	OrderStatusIn    []int  // 注文ステータス複数指定
	DeliveryStatusIn []int  // 納品ステータス複数指定
	ProjectNoEq      string // プロジェクト番号完全一致
	ManagementNoEq   string // 管理番号完全一致

	// 日付フィルタ（YYYY-MM-DD）
	DeliveryDateGteq string // 納期 >= (YYYY-MM-DD)
	DeliveryDateLteq string // 納期 <= (YYYY-MM-DD)
	InvoiceDateGteq  string // 請求日 >= (YYYY-MM-DD)
	InvoiceDateLteq  string // 請求日 <= (YYYY-MM-DD)

	// 請求タイミング
	InvoiceTimingKbnIn []int // 請求タイミング区分複数指定

	// タグ・レスポンスグループ
	Tags          []string // タグ
	ResponseGroup string   // "small" / "large" / "estimate" / "order" / etc.
}

// buildProjectsQuery builds the Ransack-style query string for
// GET /v1/projects based on the given options and page.
func buildProjectsQuery(opts ProjectListOptions, page, perPage int) string {
	return NewQueryBuilder().
		Page(page, perPage).
		StrCont("name", opts.NameCont).
		IntEq("client_id", opts.ClientIDEq).
		StrCont("client_name", opts.ClientNameCont).
		IntIn("order_status", opts.OrderStatusIn).
		IntIn("delivery_status", opts.DeliveryStatusIn).
		StrEq("project_no", opts.ProjectNoEq).
		StrEq("management_no", opts.ManagementNoEq).
		DateGteq("delivery_date", opts.DeliveryDateGteq).
		DateLteq("delivery_date", opts.DeliveryDateLteq).
		DateGteq("invoice_date", opts.InvoiceDateGteq).
		DateLteq("invoice_date", opts.InvoiceDateLteq).
		IntIn("invoice_timing_kbn", opts.InvoiceTimingKbnIn).
		DateGteq("updated_at", opts.UpdatedAtGteq).
		DateLteq("updated_at", opts.UpdatedAtLteq).
		DateGteq("created_at", opts.CreatedAtGteq).
		DateLteq("created_at", opts.CreatedAtLteq).
		Flg01("include_archive_flg", opts.IncludeArchiveFlg).
		Flg01("include_lost_flg", opts.IncludeLostFlg).
		Tags(opts.Tags).
		ResponseGroup(opts.ResponseGroup).
		Encode()
}

// ListProjects retrieves projects filtered by the given options.
// Pagination is handled internally by ListAllWithResult; metadata (total
// count, page, rate limits, ETag) is surfaced via the returned *ListResult.
//
// Pass ProjectListOptions{} for an unfiltered list of all projects.
func (c *Client) ListProjects(ctx context.Context, opts ProjectListOptions) (*ListResult[ProjectEntity], error) {
	perPage := opts.PerPage
	makeReq := func(ctx context.Context, page, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = buildProjectsQuery(opts, page, pp)
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
	items := make([]ProjectEntity, 0, len(raw.Items))
	for _, b := range raw.Items {
		var x ProjectEntity
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjects: unmarshal: " + err.Error()}
		}
		items = append(items, x)
	}
	return &ListResult[ProjectEntity]{Items: items, Meta: raw.Meta, Headers: raw.Headers}, nil
}

// GetProject retrieves the project with the specified ID and returns an
// ItemResult carrying the entity together with response metadata (ETag,
// rate limits, Last-Modified).
func (c *Client) GetProject(ctx context.Context, id int) (*ItemResult[ProjectEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProject: unmarshal: " + err.Error()}
	}
	return &ItemResult[ProjectEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// GetProjectWithGroup retrieves the project with the specified ID and response_group,
// and returns an ItemResult with metadata.
// responseGroup can be "estimate", "order", "delivery", "invoice", "receipt", or "all".
// If responseGroup is empty, behaves like GetProject.
func (c *Client) GetProjectWithGroup(ctx context.Context, id int, responseGroup string) (*ItemResult[ProjectEntity], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	if responseGroup != "" {
		req.URL.RawQuery = NewQueryBuilder().ResponseGroup(responseGroup).Encode()
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectWithGroup: unmarshal: " + err.Error()}
	}
	return &ItemResult[ProjectEntity]{
		Item:    &x,
		Meta:    parseItemMeta(headers),
		Headers: headers,
	}, nil
}

// ListProjectsRaw retrieves all projects and returns the raw HTTP response
// bodies merged across pages as a single JSON array. Unlike ListProjects, the
// returned bytes are byte-preserving: each element JSON is exactly what the
// BOARD API emitted, enabling strict field diff in E2E tests to detect keys
// that are not mapped to ProjectEntity.
//
// Intended for E2E strict field diff; regular callers should use ListProjects.
func (c *Client) ListProjectsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	result, err := c.ListAllWithResult(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(result.Items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjectsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}

// GetProjectRaw retrieves a single project and returns the raw HTTP response
// body byte-for-byte.
//
// Intended for E2E strict field diff; regular callers should use GetProject.
func (c *Client) GetProjectRaw(ctx context.Context, id int) ([]byte, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}

// GetProjectWithGroupRaw retrieves a single project with response_group and
// returns the raw HTTP response body byte-for-byte.
//
// responseGroup can be "estimate", "order", "delivery", "invoice", "receipt",
// or "all". If responseGroup is empty, behaves like GetProjectRaw (no
// response_group query is appended).
//
// Intended for E2E strict field diff; regular callers should use
// GetProjectWithGroup.
func (c *Client) GetProjectWithGroupRaw(ctx context.Context, id int, responseGroup string) ([]byte, error) {
	path := fmt.Sprintf("/v1/projects/%d", id)
	if responseGroup != "" {
		path += "?response_group=" + responseGroup
	}
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(req)
}
