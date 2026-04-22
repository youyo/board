package boardapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newProjectsMockClient returns a boardapi.Client whose HTTP client routes every
// request through rt. Mirrors newClientsMockClient (M12) /
// newProjectCostsMockClient (M11) to keep unit tests runnable in the sandbox
// where opening a local listener is denied. The shared roundTripperFunc and
// jsonResp helpers are defined in accounting_types_test.go (package-scope).
func newProjectsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// testdataPath returns the absolute path to a file in tmp/e2e-artifacts/.
// Uses runtime.Caller to locate the file relative to the test file itself.
func testdataPath(t *testing.T, filename string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile: .../internal/boardapi/projects_test.go
	// target:   .../tmp/e2e-artifacts/<filename>
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, "tmp", "e2e-artifacts", filename)
}

// readFixture reads a fixture file and returns its bytes.
func readFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := testdataPath(t, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture %s not available: %v", filename, err)
	}
	return data
}

// ===== U1-U12: M44 ProjectEntity 全面再設計 Unit テスト =====

// U1: TestProjectEntity_UnmarshalGet_AllFields
// プレーン Get 実 dump（72 フィールド）をデシリアライズし全フィールド検証。
func TestProjectEntity_UnmarshalGet_AllFields(t *testing.T) {
	data := readFixture(t, "projects_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// ID/Name
	if p.ID != 95944469 {
		t.Errorf("ID = %d, want 95944469", p.ID)
	}
	if p.Name == "" {
		t.Error("Name should not be empty")
	}

	// Client (nested)
	if p.Client == nil {
		t.Fatal("Client should not be nil")
	}
	if p.Client.ID != 53661066 {
		t.Errorf("Client.ID = %d, want 53661066", p.Client.ID)
	}
	if p.Client.Name == "" {
		t.Error("Client.Name should not be empty")
	}

	// User (nested)
	if p.User == nil {
		t.Fatal("User should not be nil")
	}
	if p.User.LastName == "" {
		t.Error("User.LastName should not be empty")
	}

	// Hubspot (nested, hubspot_deal_id: null)
	if p.Hubspot == nil {
		t.Fatal("Hubspot should not be nil (even if hubspot_deal_id is null)")
	}
	if p.Hubspot.HubspotDealID != nil {
		t.Errorf("Hubspot.HubspotDealID = %v, want nil", *p.Hubspot.HubspotDealID)
	}

	// null nested (Risk-3)
	if p.ClientBranch != nil {
		t.Error("ClientBranch should be nil (dump was null)")
	}
	if p.Contact != nil {
		t.Error("Contact should be nil (dump was null)")
	}
	if p.CompanyBranch != nil {
		t.Error("CompanyBranch should be nil (dump was null)")
	}

	// Total / Tax (string 型)
	if p.Total == nil || *p.Total == "" {
		t.Error("Total should not be nil/empty")
	}
	if p.Tax == nil || *p.Tax == "" {
		t.Error("Tax should not be nil/empty")
	}

	// CostTotal / CostTax (int 型) — 型混在確認
	// dump: cost_total=0, cost_tax=0
	_ = p.CostTotal
	_ = p.CostTax

	// InvoiceTotal / InvoiceTax (string 型)
	if p.InvoiceTotal == nil || *p.InvoiceTotal == "" {
		t.Error("InvoiceTotal should not be nil/empty")
	}

	// OrderStatus / DeliveryStatus
	if p.OrderStatus == 0 {
		t.Error("OrderStatus should not be 0")
	}
	if p.OrderStatusName == "" {
		t.Error("OrderStatusName should not be empty")
	}
	if p.DeliveryStatus == 0 {
		t.Error("DeliveryStatus should not be 0")
	}

	// ProjectTypeID (not nil)
	if p.ProjectTypeID == nil {
		t.Error("ProjectTypeID should not be nil")
	}

	// Tags (空配列として帰ってくる)
	if p.Tags == nil {
		t.Error("Tags should be non-nil (empty slice)")
	}

	// InvoiceDates
	if len(p.InvoiceDates) == 0 {
		t.Error("InvoiceDates should not be empty")
	}

	// ContractStartDate / ContractEndDate (旧 StartDate/EndDate の代替)
	if p.ContractStartDate == nil {
		t.Error("ContractStartDate should not be nil")
	}
	if p.ContractEndDate == nil {
		t.Error("ContractEndDate should not be nil")
	}

	// Hubspot not nil but deal ID is nil
	if p.Hubspot == nil {
		t.Error("Hubspot should not be nil")
	}

	// DocumentSettingID not nil
	if p.DocumentSettingID == nil {
		t.Error("DocumentSettingID should not be nil")
	}

	// UpdatedAt / CreatedAt
	if p.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
	if p.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}

	// response_group フィールドは未指定時 nil
	if p.Estimate != nil {
		t.Error("Estimate should be nil for plain Get")
	}
	if p.Order != nil {
		t.Error("Order should be nil for plain Get")
	}
}

// U2: TestProjectEntity_UnmarshalList_SparseFields
// List レスポンスの 1 件（25 キーサブセット）をデシリアライズし、
// Get 限定フィールドが zero-value/nil であることを確認。
func TestProjectEntity_UnmarshalList_SparseFields(t *testing.T) {
	// List レスポンスの 1 件（実 dump からサブセット）
	listItem := `{
		"id": 95944469,
		"project_no": 4106,
		"management_no": null,
		"name": "テストプロジェクト",
		"client": {"id": 53661066, "name": "テスト会社", "name_disp": "テスト", "custom_no": ""},
		"contact": null,
		"user": {"id": 38516996, "last_name": "立花", "first_name": "拓也"},
		"total": "90000.0",
		"tax": "9000.0",
		"estimate_date": "2026-04-17",
		"invoice_dates": ["2026-06-30"],
		"order_status": 2,
		"order_status_name": "見積中(中)",
		"delivery_status": 1,
		"delivery_status_name": "未着手",
		"project_type_id": 48728974,
		"project_type_name": "マネージド",
		"project_type2_id": null,
		"project_type2_name": null,
		"project_type3_id": null,
		"project_type3_name": null,
		"group_id": null,
		"group_name": null,
		"created_at": "2026-04-17T16:28:03.000+09:00",
		"updated_at": "2026-04-17T17:10:32.000+09:00"
	}`

	var p boardapi.ProjectEntity
	if err := json.Unmarshal([]byte(listItem), &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// 共通フィールドが埋まる
	if p.ID != 95944469 {
		t.Errorf("ID = %d, want 95944469", p.ID)
	}
	if p.OrderStatus != 2 {
		t.Errorf("OrderStatus = %d, want 2", p.OrderStatus)
	}

	// Get 限定フィールドは zero/nil
	if p.ClientNameDispKbn != 0 {
		t.Errorf("ClientNameDispKbn = %d, want 0 (List sparse)", p.ClientNameDispKbn)
	}
	if p.InvoiceTimingKbn != 0 {
		t.Errorf("InvoiceTimingKbn = %d, want 0 (List sparse)", p.InvoiceTimingKbn)
	}
	if p.Hubspot != nil {
		t.Error("Hubspot should be nil in List response")
	}

	// ProjectCosts は nil
	if p.ProjectCosts != nil {
		t.Error("ProjectCosts should be nil in List response")
	}
}

// U3: TestProjectEntity_UnmarshalRgEstimate
// rg=estimate dump をデシリアライズし、Estimate フィールドが正しく埋まることを確認。
func TestProjectEntity_UnmarshalRgEstimate(t *testing.T) {
	data := readFixture(t, "projects_rg_estimate_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Estimate != nil
	if p.Estimate == nil {
		t.Fatal("Estimate should not be nil for rg=estimate")
	}

	// ValidPeriod
	if p.Estimate.ValidPeriod == nil {
		t.Error("Estimate.ValidPeriod should not be nil")
	} else if *p.Estimate.ValidPeriod == "" {
		t.Error("Estimate.ValidPeriod should not be empty")
	}

	// Details 存在
	if len(p.Estimate.Details) == 0 {
		t.Error("Estimate.Details should not be empty")
	}
	if p.Estimate.Details[0].No == 0 {
		t.Error("Estimate.Details[0].No should not be 0")
	}

	// Order / Deliveries / Invoices / Receipts は nil/empty
	if p.Order != nil {
		t.Error("Order should be nil for rg=estimate")
	}
	if len(p.Deliveries) != 0 {
		t.Errorf("Deliveries should be empty for rg=estimate, got %d", len(p.Deliveries))
	}
	if len(p.Invoices) != 0 {
		t.Errorf("Invoices should be empty for rg=estimate, got %d", len(p.Invoices))
	}
	if len(p.Receipts) != 0 {
		t.Errorf("Receipts should be empty for rg=estimate, got %d", len(p.Receipts))
	}
}

// U4: TestProjectEntity_UnmarshalRgOrder
// rg=order dump をデシリアライズし、Order フィールドが正しく埋まることを確認。
func TestProjectEntity_UnmarshalRgOrder(t *testing.T) {
	data := readFixture(t, "projects_rg_order_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Order != nil
	if p.Order == nil {
		t.Fatal("Order should not be nil for rg=order")
	}

	// Details 存在
	if len(p.Order.Details) == 0 {
		t.Error("Order.Details should not be empty")
	}

	// DispOrderDate or DispOrderReceiveDate (optional, may be nil)
	// type-specific フィールドが unmarshal されること
	_ = p.Order.DispOrderDate
	_ = p.Order.DispOrderReceiveDate

	// Estimate は nil
	if p.Estimate != nil {
		t.Error("Estimate should be nil for rg=order")
	}
}

// U5: TestProjectEntity_UnmarshalRgDelivery
// rg=delivery dump をデシリアライズし、Deliveries 配列が正しく埋まることを確認。
func TestProjectEntity_UnmarshalRgDelivery(t *testing.T) {
	data := readFixture(t, "projects_rg_delivery_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Deliveries 1 件
	if len(p.Deliveries) == 0 {
		t.Fatal("Deliveries should not be empty for rg=delivery")
	}

	d := p.Deliveries[0]
	// DeliveryDate
	if d.DeliveryDate == nil {
		t.Error("Deliveries[0].DeliveryDate should not be nil")
	} else if *d.DeliveryDate == "" {
		t.Error("Deliveries[0].DeliveryDate should not be empty")
	}

	// DeliveryPlace
	if d.DeliveryPlace == nil {
		t.Error("Deliveries[0].DeliveryPlace should not be nil")
	}

	// Details 存在
	if len(d.Details) == 0 {
		t.Error("Deliveries[0].Details should not be empty")
	}
}

// U6: TestProjectEntity_UnmarshalRgInvoice
// rg=invoice dump をデシリアライズし、Invoices 配列が正しく埋まることを確認。
func TestProjectEntity_UnmarshalRgInvoice(t *testing.T) {
	data := readFixture(t, "projects_rg_invoice_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Invoices 3 件
	if len(p.Invoices) == 0 {
		t.Fatal("Invoices should not be empty for rg=invoice")
	}

	inv := p.Invoices[0]
	// InvoiceDate
	if inv.InvoiceDate == nil {
		t.Error("Invoices[0].InvoiceDate should not be nil")
	}
	// PaymentLimitDate
	if inv.PaymentLimitDate == nil {
		t.Error("Invoices[0].PaymentLimitDate should not be nil")
	}
}

// U7: TestProjectEntity_UnmarshalRgReceipt
// rg=receipt dump をデシリアライズし、Receipts 配列が正しく埋まることを確認。
func TestProjectEntity_UnmarshalRgReceipt(t *testing.T) {
	data := readFixture(t, "projects_rg_receipt_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Receipts 3 件
	if len(p.Receipts) == 0 {
		t.Fatal("Receipts should not be empty for rg=receipt")
	}

	r := p.Receipts[0]
	// ReceiptDate
	if r.ReceiptDate == nil {
		t.Error("Receipts[0].ReceiptDate should not be nil")
	}
}

// U8: TestProjectEntity_UnmarshalRgAll
// rg=all dump をデシリアライズし、全サブドキュメントが存在することを確認。
func TestProjectEntity_UnmarshalRgAll(t *testing.T) {
	data := readFixture(t, "projects_rg_all_95944469.json")

	var p boardapi.ProjectEntity
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if p.Estimate == nil {
		t.Error("Estimate should not be nil for rg=all")
	}
	if p.Order == nil {
		t.Error("Order should not be nil for rg=all")
	}
	if len(p.Deliveries) == 0 {
		t.Error("Deliveries should not be empty for rg=all")
	}
	if len(p.Invoices) == 0 {
		t.Error("Invoices should not be empty for rg=all")
	}
	if len(p.Receipts) == 0 {
		t.Error("Receipts should not be empty for rg=all")
	}

	// ProjectCosts は json.RawMessage（空配列 [] でも non-nil）
	// rg=all 時のみ出現、中身は空配列が観測されている
	_ = p.ProjectCosts
}

// U9: TestProjectEntity_NullableNested_NilForNull
// null の nested フィールドが正しく nil となることを確認。
func TestProjectEntity_NullableNested_NilForNull(t *testing.T) {
	input := `{
		"id": 1,
		"name": "テスト",
		"client_branch": null,
		"contact": null,
		"company_branch": null,
		"invoice_dates": [],
		"tags": [],
		"updated_at": "2024-01-01T00:00:00+09:00",
		"created_at": "2024-01-01T00:00:00+09:00"
	}`

	var p boardapi.ProjectEntity
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("Unmarshal error (should not panic): %v", err)
	}

	if p.ClientBranch != nil {
		t.Error("ClientBranch should be nil for null")
	}
	if p.Contact != nil {
		t.Error("Contact should be nil for null")
	}
	if p.CompanyBranch != nil {
		t.Error("CompanyBranch should be nil for null")
	}
}

// U10: TestProjectSearchParams_QueryEncoding
// ProjectSearchParams の全 5 パラメータが正しくクエリエンコードされることを確認。
func TestProjectSearchParams_QueryEncoding(t *testing.T) {
	page1 := `[{"id":7,"name":"matched","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedClientID, observedName, observedStatus, observedUpdatedFrom, observedResponseGroup string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedClientID = r.URL.Query().Get("client_id")
		observedName = r.URL.Query().Get("name")
		observedStatus = r.URL.Query().Get("status")
		observedUpdatedFrom = r.URL.Query().Get("updated_at_from")
		observedResponseGroup = r.URL.Query().Get("response_group")
		return jsonResp(page1), nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.SearchProjectsRaw(context.Background(), boardapi.ProjectSearchParams{
		ClientID:      100,
		Name:          "matched",
		Status:        "active",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
		ResponseGroup: "estimate",
	})
	if err != nil {
		t.Fatalf("SearchProjectsRaw: %v", err)
	}
	if observedClientID != "100" {
		t.Errorf("query client_id = %q, want 100", observedClientID)
	}
	if observedName != "matched" {
		t.Errorf("query name = %q, want matched", observedName)
	}
	if observedStatus != "active" {
		t.Errorf("query status = %q, want active", observedStatus)
	}
	if observedUpdatedFrom != "2024-01-01T00:00:00+09:00" {
		t.Errorf("query updated_at_from = %q, want 2024-01-01T00:00:00+09:00", observedUpdatedFrom)
	}
	if observedResponseGroup != "estimate" {
		t.Errorf("query response_group = %q, want estimate", observedResponseGroup)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}

// U11: TestDocumentDetail_NullableNumeric
// DocumentDetailEntity の nullable string フィールドが null の場合に正しく nil となることを確認。
func TestDocumentDetail_NullableNumeric(t *testing.T) {
	input := `{
		"no": 1,
		"detail_date": null,
		"description": "テスト明細",
		"quantity": null,
		"unit": null,
		"unit_price": null,
		"price": null,
		"tax_rate": "10.0",
		"tax_withholding_flg": 0,
		"tax_included_flg": 0,
		"reduced_tax_rate_kbn": 1,
		"section_description": null,
		"section_subtotal": null,
		"document_detail_kbn": 1,
		"document_detail_kbn_name": "通常",
		"deduction_applicable": false
	}`

	var d boardapi.DocumentDetailEntity
	if err := json.Unmarshal([]byte(input), &d); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if d.Quantity != nil {
		t.Errorf("Quantity = %v, want nil", *d.Quantity)
	}
	if d.Price != nil {
		t.Errorf("Price = %v, want nil", *d.Price)
	}
	if d.UnitPrice != nil {
		t.Errorf("UnitPrice = %v, want nil", *d.UnitPrice)
	}
	if d.No != 1 {
		t.Errorf("No = %d, want 1", d.No)
	}
}

// U12: TestProjectEntity_TagsEmptyArray
// tags が [] の場合に Tags は nil ではなく空スライスであることを確認。
func TestProjectEntity_TagsEmptyArray(t *testing.T) {
	input := `{
		"id": 1,
		"name": "テスト",
		"tags": [],
		"invoice_dates": [],
		"order_status": 1,
		"order_status_name": "受注",
		"delivery_status": 1,
		"delivery_status_name": "未着手",
		"updated_at": "2024-01-01T00:00:00+09:00",
		"created_at": "2024-01-01T00:00:00+09:00"
	}`

	var p boardapi.ProjectEntity
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if p.Tags == nil {
		t.Error("Tags should be non-nil (empty slice, not nil)")
	}
	if len(p.Tags) != 0 {
		t.Errorf("Tags len = %d, want 0", len(p.Tags))
	}
}

// ===== 既存テスト（M44 新 schema 対応版）=====

// TestListProjectsRaw_SinglePage: ListProjectsRaw returns the raw JSON array
// body byte-for-byte when a single page response is served.
// M44: JSON 文字列を新スキーマ（order_status/delivery_status 等）に更新。
func TestListProjectsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"name":"Project A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.ListProjectsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListProjectsRaw: %v", err)
	}
	if gotPath != "/v1/projects" {
		t.Errorf("path = %s, want /v1/projects", gotPath)
	}
	if gotQuery.Get("page") != "1" {
		t.Errorf("page = %s, want 1", gotQuery.Get("page"))
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v\nraw=%s", err, string(raw))
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
	got := arr[0]
	// 新スキーマの必須キー
	wantKeys := []string{
		"id", "name", "order_status", "order_status_name",
		"delivery_status", "delivery_status_name", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// TestListProjectsRaw_MultiPage: multiple pages are concatenated into a single JSON array.
func TestListProjectsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"name":"B","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"name":"C","order_status":2,"order_status_name":"見積中","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
	}
	var pageCount int
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		pageCount++
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("per_page")
		if perPage != "2" {
			t.Errorf("expected per_page=2, got %s", perPage)
		}
		switch page {
		case "1":
			return jsonResp(fmt.Sprintf("[%s]", strings.Join(page1Items, ","))), nil
		case "2":
			return jsonResp(fmt.Sprintf("[%s]", strings.Join(page2Items, ","))), nil
		default:
			t.Errorf("unexpected page: %s", page)
			return jsonResp("[]"), nil
		}
	})
	client := newProjectsMockClient(rt)

	raw, err := client.ListProjectsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListProjectsRaw: %v", err)
	}
	if pageCount != 2 {
		t.Errorf("expected 2 page requests, got %d", pageCount)
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v\nraw=%s", err, string(raw))
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	for i, want := range []float64{1, 2, 3} {
		if got, _ := arr[i]["id"].(float64); got != want {
			t.Errorf("arr[%d].id = %v, want %v", i, got, want)
		}
	}
}

// TestGetProjectRaw_Success: GetProjectRaw returns body exactly as served.
func TestGetProjectRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"name":"Project X","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.GetProjectRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetProjectRaw: %v", err)
	}
	if gotPath != "/v1/projects/42" {
		t.Errorf("path = %s, want /v1/projects/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// TestGetProjectRaw_NotFound: GetProjectRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetProjectRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newProjectsMockClient(rt)

	_, err := client.GetProjectRaw(context.Background(), 99)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("APIError.Code = %v, want %v", apiErr.Code, boardapi.APIErrorNotFound)
	}
}

// TestGetProjectWithGroupRaw_QueryParam: response_group is sent as a query parameter.
func TestGetProjectWithGroupRaw_QueryParam(t *testing.T) {
	body := []byte(`{"id":42,"name":"P","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"","created_at":""}`)
	groups := []string{"estimate", "order", "delivery", "invoice", "receipt", "all"}
	for _, group := range groups {
		t.Run(group, func(t *testing.T) {
			var gotPath, gotRawQuery string
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				gotRawQuery = r.URL.RawQuery
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			})
			client := newProjectsMockClient(rt)
			_, err := client.GetProjectWithGroupRaw(context.Background(), 42, group)
			if err != nil {
				t.Fatalf("GetProjectWithGroupRaw(%s): %v", group, err)
			}
			if gotPath != "/v1/projects/42" {
				t.Errorf("path = %s, want /v1/projects/42", gotPath)
			}
			wantQuery := "response_group=" + group
			if gotRawQuery != wantQuery {
				t.Errorf("raw query = %q, want %q", gotRawQuery, wantQuery)
			}
		})
	}

	// Empty response_group: no query parameter must be appended.
	t.Run("empty", func(t *testing.T) {
		var gotPath, gotRawQuery string
		rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotRawQuery = r.URL.RawQuery
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		})
		client := newProjectsMockClient(rt)
		_, err := client.GetProjectWithGroupRaw(context.Background(), 42, "")
		if err != nil {
			t.Fatalf("GetProjectWithGroupRaw(empty): %v", err)
		}
		if gotPath != "/v1/projects/42" {
			t.Errorf("path = %s, want /v1/projects/42", gotPath)
		}
		if gotRawQuery != "" {
			t.Errorf("raw query = %q, want empty (no response_group when arg is empty)", gotRawQuery)
		}
	})
}
