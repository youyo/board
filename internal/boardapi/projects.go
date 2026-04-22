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

// ProjectSearchParams は SearchProjects のパラメータ。
// NOTE: BOARD API の name フィルタおよび status フィルタは実測で無視されることが
// コンプライアンスロードマップの 9 件連続観測で確認されている。
// 保持するが期待どおりのフィルタリングが実施されない可能性がある。
type ProjectSearchParams struct {
	ClientID      int
	Name          string
	Status        string
	UpdatedAtFrom string
	ResponseGroup string
}

// ListProjects retrieves all projects.
// Pagination is automatically handled by ListAll.
func (c *Client) ListProjects(ctx context.Context) ([]ProjectEntity, error) {
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
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "ListProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// GetProject retrieves the project with the specified ID.
func (c *Client) GetProject(ctx context.Context, id int) (*ProjectEntity, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/projects/%d", id), nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProject: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// GetProjectWithGroup retrieves the project with the specified ID and response_group.
// responseGroup can be "estimate", "order", "delivery", "invoice", "receipt", or "all".
// If responseGroup is empty, behaves like GetProject.
func (c *Client) GetProjectWithGroup(ctx context.Context, id int, responseGroup string) (*ProjectEntity, error) {
	path := fmt.Sprintf("/v1/projects/%d", id)
	if responseGroup != "" {
		path += "?response_group=" + responseGroup
	}
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	body, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	var x ProjectEntity
	if err := json.Unmarshal(body, &x); err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "GetProjectWithGroup: unmarshal: " + err.Error()}
	}
	return &x, nil
}

// SearchProjects searches projects with the given conditions.
// Pagination is automatically handled by ListAll.
func (c *Client) SearchProjects(ctx context.Context, params ProjectSearchParams) ([]ProjectEntity, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
		}
		if params.Name != "" {
			q.Set("name", params.Name)
		}
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		if params.ResponseGroup != "" {
			q.Set("response_group", params.ResponseGroup)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq)
	if err != nil {
		return nil, err
	}
	result := make([]ProjectEntity, 0, len(items))
	for _, raw := range items {
		var x ProjectEntity
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjects: unmarshal: " + err.Error()}
		}
		result = append(result, x)
	}
	return result, nil
}

// ListProjectsPage retrieves a single page of projects.
func (c *Client) ListProjectsPage(ctx context.Context, page, perPage int) (*PageResult[ProjectEntity], error) {
	makeReq := func(ctx context.Context, p, pp int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(p))
		q.Set("per_page", strconv.Itoa(pp))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	return ListPage[ProjectEntity](c, ctx, makeReq, page, perPage)
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
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
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
// The query is concatenated directly to the path via "?response_group=<value>"
// to mirror GetProjectWithGroup's existing implementation; BOARD API has been
// validated to parse this form. Raw variant is exposed for E2E strict field
// diff on DocumentSummary sub-entities (estimate/order/delivery/invoice/
// receipt) which appear only when response_group is specified.
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

// SearchProjectsRaw retrieves projects matching the given search parameters
// and returns the raw HTTP response bodies merged across pages as a single
// JSON array. Same byte-preserving guarantee as ListProjectsRaw.
//
// ProjectSearchParams exposes 5 filters (ClientID, Name, Status, UpdatedAtFrom,
// ResponseGroup) — the richest surface across M02-M12. Note that the BOARD
// API has been observed to ignore the `name` filter across 7 consecutive
// milestones (M03/M04/M06/M08/M09/M10/M12), so the Name value in Search only
// exercises request encoding, not server-side filtering.
//
// Intended for E2E strict field diff; regular callers should use
// SearchProjects.
func (c *Client) SearchProjectsRaw(ctx context.Context, params ProjectSearchParams, opts ...ListAllOption) ([]byte, error) {
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := c.NewRequest(ctx, http.MethodGet, "/v1/projects", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		if params.ClientID != 0 {
			q.Set("client_id", strconv.Itoa(params.ClientID))
		}
		if params.Name != "" {
			q.Set("name", params.Name)
		}
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.UpdatedAtFrom != "" {
			q.Set("updated_at_from", params.UpdatedAtFrom)
		}
		if params.ResponseGroup != "" {
			q.Set("response_group", params.ResponseGroup)
		}
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	items, err := c.ListAll(ctx, makeReq, opts...)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(items)
	if err != nil {
		return nil, &APIError{Code: APIErrorUnknown, Message: "SearchProjectsRaw: marshal aggregate: " + err.Error()}
	}
	return out, nil
}
