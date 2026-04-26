package find2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// newPOTestService は FindPurchaseOrder テスト用の Service を生成するヘルパー。
func newPOTestService(
	pos *stubPurchaseOrderRepo,
	vendors *stubVendorRepo,
	projects *stubProjectRepo,
) *Service {
	r := newTestRepos()
	r.PurchaseOrders = pos
	r.Vendors = vendors
	r.Projects = projects
	return New(r)
}

// P01: ByID HappyPath（VendorID/ProjectID enrichment 成功）
func TestService_FindPurchaseOrder_ByID_HappyPath(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		getResult: &boardapi.PurchaseOrderEntity{ID: 100, VendorID: 5, ProjectID: 7, Title: "PO-100"},
	}
	vendors := &stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 5, Name: "V"}}
	projects := &stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 7, Name: "P"}}
	svc := newPOTestService(pos, vendors, projects)

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.PurchaseOrder.ID != 100 {
		t.Errorf("PurchaseOrder.ID=%d, want 100", r.PurchaseOrder.ID)
	}
	if r.Vendor == nil || r.Vendor.ID != 5 {
		t.Errorf("Vendor mismatch: %+v", r.Vendor)
	}
	if r.Project == nil || r.Project.ID != 7 {
		t.Errorf("Project mismatch: %+v", r.Project)
	}
}

// P02: ByID — Project enrichment 失敗 → non-fatal（Project=nil + slog.Warn）
func TestService_FindPurchaseOrder_ByID_ProjectEnrichmentFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	pos := &stubPurchaseOrderRepo{
		getResult: &boardapi.PurchaseOrderEntity{ID: 100, VendorID: 5, ProjectID: 7},
	}
	vendors := &stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 5}}
	projects := &stubProjectRepo{
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, errors.New("project fetch failed")
		},
	}
	svc := newPOTestService(pos, vendors, projects)

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project != nil {
		t.Errorf("Project should be nil after fail, got %+v", results[0].Project)
	}
	if results[0].Vendor == nil || results[0].Vendor.ID != 5 {
		t.Errorf("Vendor should succeed: %+v", results[0].Vendor)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn record, got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "project enrichment failed") {
		t.Errorf("unexpected slog message: %q", warns[0].Message)
	}
}

// P03: ByID — Statuses post-filter は ID 検索時 skip
func TestService_FindPurchaseOrder_ByID_StatusesPostFilterSkipped(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		getResult: &boardapi.PurchaseOrderEntity{ID: 100, Status: "archived"},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{ID: 100, Statuses: []string{"sent"}})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (status filter skipped on ID), got %d", len(results))
	}
}

// P04: ByVendorID — VendorIDEq が API に渡る、Status="" の場合 StatusEq も ""
func TestService_FindPurchaseOrder_ByVendorID_DelegatesFilter(t *testing.T) {
	var captured boardapi.PurchaseOrderListOptions
	pos := &stubPurchaseOrderRepo{
		searchFunc: func(_ context.Context, f boardapi.PurchaseOrderListOptions, _ repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
			captured = f
			return []boardapi.PurchaseOrderEntity{{ID: 1}, {ID: 2}}, nil
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{VendorID: 5})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if captured.VendorIDEq != 5 {
		t.Errorf("VendorIDEq=%d, want 5", captured.VendorIDEq)
	}
	if captured.StatusEq != "" {
		t.Errorf("StatusEq=%q, want empty", captured.StatusEq)
	}
}

// P05: ByVendorID + Status — VendorIDEq + StatusEq 両方 API 委譲
func TestService_FindPurchaseOrder_ByVendorIDAndStatus_DelegatesBoth(t *testing.T) {
	var captured boardapi.PurchaseOrderListOptions
	pos := &stubPurchaseOrderRepo{
		searchFunc: func(_ context.Context, f boardapi.PurchaseOrderListOptions, _ repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
			captured = f
			return []boardapi.PurchaseOrderEntity{{ID: 1}}, nil
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	_, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{VendorID: 5, Status: "approved"})
	assertNoError(t, err)
	if captured.VendorIDEq != 5 || captured.StatusEq != "approved" {
		t.Errorf("filter mismatch: %+v", captured)
	}
}

// P06: ByStatus（単独）— Status only は allow、StatusEq が API に渡る
func TestService_FindPurchaseOrder_ByStatus_Only_AllowedAndDelegated(t *testing.T) {
	var captured boardapi.PurchaseOrderListOptions
	pos := &stubPurchaseOrderRepo{
		searchFunc: func(_ context.Context, f boardapi.PurchaseOrderListOptions, _ repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
			captured = f
			return []boardapi.PurchaseOrderEntity{{ID: 1, Status: "approved"}}, nil
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{Status: "approved"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if captured.StatusEq != "approved" {
		t.Errorf("StatusEq=%q, want 'approved'", captured.StatusEq)
	}
	if captured.VendorIDEq != 0 {
		t.Errorf("VendorIDEq should be 0, got %d", captured.VendorIDEq)
	}
}

// P07: ByText — Title マッチ
func TestService_FindPurchaseOrder_ByText_MatchesTitle(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{
			{ID: 1, Title: "Acme PO"},
			{ID: 2, Title: "Beta PO"},
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{Text: "acme"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// P08: ByText — Memo マッチ
func TestService_FindPurchaseOrder_ByText_MatchesMemo(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{
			{ID: 1, Memo: "urgent procurement"},
			{ID: 2, Memo: "normal"},
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{Text: "urgent"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// P09: ByStatuses (multi) + VendorID — post-filter
func TestService_FindPurchaseOrder_ByStatuses_PostFilters(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{
			{ID: 1, Status: "sent"},
			{ID: 2, Status: "draft"},
			{ID: 3, Status: "approved"},
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{
		VendorID: 5,
		Statuses: []string{"sent", "approved"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// P10: Limit=2
func TestService_FindPurchaseOrder_LimitTwo_StopsEnrichment(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{
			{ID: 1, VendorID: 5},
			{ID: 2, VendorID: 5},
			{ID: 3, VendorID: 5},
		},
	}
	vendors := &stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 5}}
	svc := newPOTestService(pos, vendors, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{
		VendorID:       5,
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// P11: Empty query → error
func TestService_FindPurchaseOrder_EmptyQuery_Error(t *testing.T) {
	svc := newPOTestService(&stubPurchaseOrderRepo{}, &stubVendorRepo{}, &stubProjectRepo{})

	_, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// P12: Statuses-only → reject
func TestService_FindPurchaseOrder_StatusesOnly_RejectedByValidate(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{{ID: 1}},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	_, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{Statuses: []string{"sent"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Statuses requires one of") {
		t.Errorf("unexpected error message: %v", err)
	}
	if pos.searchCount != 0 {
		t.Errorf("Search should not be called on validation error, got %d", pos.searchCount)
	}
}

// P09b: ByText + Statuses — Text マッチしたサブセットに Statuses post-filter
func TestService_FindPurchaseOrder_ByTextAndStatuses_PostFiltersTextMatched(t *testing.T) {
	pos := &stubPurchaseOrderRepo{
		searchResult: []boardapi.PurchaseOrderEntity{
			{ID: 1, Title: "Urgent A", Status: "sent"},
			{ID: 2, Title: "Urgent B", Status: "draft"},
			{ID: 3, Title: "Urgent C", Status: "approved"},
			{ID: 4, Title: "Normal", Status: "sent"},
		},
	}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	results, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{
		Text:     "urgent",
		Statuses: []string{"sent", "approved"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Text=urgent ∩ Statuses={sent,approved}), got %d", len(results))
	}
}

// P13: GetByID error → fail-fast
func TestService_FindPurchaseOrder_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("po API error")
	pos := &stubPurchaseOrderRepo{err: fakeErr}
	svc := newPOTestService(pos, &stubVendorRepo{}, &stubProjectRepo{})

	_, err := svc.FindPurchaseOrder(testCtx, FindPurchaseOrderQuery{ID: 100})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}
