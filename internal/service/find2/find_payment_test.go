package find2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// newPaymentTestService は FindPayment テスト用の Service を生成するヘルパー。
func newPaymentTestService(
	payments *stubPaymentRepo,
	vendors *stubVendorRepo,
) *Service {
	r := newTestRepos()
	r.Payments = payments
	r.Vendors = vendors
	return New(r)
}

// M01: ByID HappyPath — Vendor enrichment 成功、Project は nil（D1）
func TestService_FindPayment_ByID_HappyPath_ProjectAlwaysNil(t *testing.T) {
	payments := &stubPaymentRepo{
		getResult: &boardapi.PaymentEntity{ID: 100, VendorID: 5, PurchaseOrderID: 99, Memo: "x"},
	}
	vendors := &stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 5, Name: "V"}}
	svc := newPaymentTestService(payments, vendors)

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Payment.ID != 100 {
		t.Errorf("Payment.ID=%d, want 100", r.Payment.ID)
	}
	if r.Vendor == nil || r.Vendor.ID != 5 {
		t.Errorf("Vendor mismatch: %+v", r.Vendor)
	}
	// D1: Project は常に nil
	if r.Project != nil {
		t.Errorf("Project must always be nil (D1), got %+v", r.Project)
	}
}

// M02: ByID — Vendor enrichment 失敗 → non-fatal + slog.Warn
func TestService_FindPayment_ByID_VendorEnrichmentFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	payments := &stubPaymentRepo{
		getResult: &boardapi.PaymentEntity{ID: 100, VendorID: 5},
	}
	vendors := &stubVendorRepo{err: errors.New("vendor fetch failed")}
	svc := newPaymentTestService(payments, vendors)

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Vendor != nil {
		t.Errorf("Vendor should be nil after fail, got %+v", results[0].Vendor)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn record, got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "vendor enrichment failed") {
		t.Errorf("unexpected slog message: %q", warns[0].Message)
	}
}

// M02b: VendorID=0 のとき vendors.GetByID は呼ばれない
func TestService_FindPayment_NoVendorID_SkipsEnrichment(t *testing.T) {
	payments := &stubPaymentRepo{
		getResult: &boardapi.PaymentEntity{ID: 100, VendorID: 0},
	}
	vendors := &stubVendorRepo{}
	svc := newPaymentTestService(payments, vendors)

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Vendor != nil {
		t.Errorf("Vendor should be nil when VendorID=0, got %+v", results[0].Vendor)
	}
}

// M03: ByVendorID — VendorIDEq が API に渡る
func TestService_FindPayment_ByVendorID_DelegatesFilter(t *testing.T) {
	var captured boardapi.PaymentListOptions
	payments := &stubPaymentRepo{
		searchFunc: func(_ context.Context, f boardapi.PaymentListOptions, _ repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
			captured = f
			return []boardapi.PaymentEntity{{ID: 1}, {ID: 2}}, nil
		},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{VendorID: 5})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if captured.VendorIDEq != 5 {
		t.Errorf("VendorIDEq=%d, want 5", captured.VendorIDEq)
	}
}

// M04: ByStatus — StatusEq API 委譲
func TestService_FindPayment_ByStatus_Only_AllowedAndDelegated(t *testing.T) {
	var captured boardapi.PaymentListOptions
	payments := &stubPaymentRepo{
		searchFunc: func(_ context.Context, f boardapi.PaymentListOptions, _ repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
			captured = f
			return []boardapi.PaymentEntity{{ID: 1, Status: "paid"}}, nil
		},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{Status: "paid"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if captured.StatusEq != "paid" {
		t.Errorf("StatusEq=%q, want 'paid'", captured.StatusEq)
	}
}

// M05: ByText — Memo マッチ（Title 無し）
func TestService_FindPayment_ByText_MatchesMemo(t *testing.T) {
	payments := &stubPaymentRepo{
		searchResult: []boardapi.PaymentEntity{
			{ID: 1, Memo: "urgent payment"},
			{ID: 2, Memo: "regular"},
		},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{Text: "urgent"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// M06: ByStatuses + VendorID — post-filter
func TestService_FindPayment_ByStatuses_PostFilters(t *testing.T) {
	payments := &stubPaymentRepo{
		searchResult: []boardapi.PaymentEntity{
			{ID: 1, Status: "paid"},
			{ID: 2, Status: "pending"},
			{ID: 3, Status: "failed"},
		},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{
		VendorID: 5,
		Statuses: []string{"paid", "failed"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// M07: Limit=2
func TestService_FindPayment_LimitTwo_StopsEnrichment(t *testing.T) {
	payments := &stubPaymentRepo{
		searchResult: []boardapi.PaymentEntity{
			{ID: 1, VendorID: 5},
			{ID: 2, VendorID: 5},
			{ID: 3, VendorID: 5},
		},
	}
	vendors := &stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 5}}
	svc := newPaymentTestService(payments, vendors)

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{
		VendorID:       5,
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// M08: Empty query → error
func TestService_FindPayment_EmptyQuery_Error(t *testing.T) {
	svc := newPaymentTestService(&stubPaymentRepo{}, &stubVendorRepo{})

	_, err := svc.FindPayment(testCtx, FindPaymentQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// M09: Statuses-only → reject
func TestService_FindPayment_StatusesOnly_RejectedByValidate(t *testing.T) {
	payments := &stubPaymentRepo{
		searchResult: []boardapi.PaymentEntity{{ID: 1}},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	_, err := svc.FindPayment(testCtx, FindPaymentQuery{Statuses: []string{"paid"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Statuses requires one of") {
		t.Errorf("unexpected error message: %v", err)
	}
	if payments.searchCount != 0 {
		t.Errorf("Search should not be called on validation error, got %d", payments.searchCount)
	}
}

// M06b: ByText + Statuses — Text マッチしたサブセットに Statuses post-filter
func TestService_FindPayment_ByTextAndStatuses_PostFiltersTextMatched(t *testing.T) {
	payments := &stubPaymentRepo{
		searchResult: []boardapi.PaymentEntity{
			{ID: 1, Memo: "urgent A", Status: "paid"},
			{ID: 2, Memo: "urgent B", Status: "pending"},
			{ID: 3, Memo: "urgent C", Status: "failed"},
			{ID: 4, Memo: "regular", Status: "paid"},
		},
	}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	results, err := svc.FindPayment(testCtx, FindPaymentQuery{
		Text:     "urgent",
		Statuses: []string{"paid", "failed"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Text=urgent ∩ Statuses={paid,failed}), got %d", len(results))
	}
}

// M10: GetByID error → fail-fast
func TestService_FindPayment_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("payment API error")
	payments := &stubPaymentRepo{err: fakeErr}
	svc := newPaymentTestService(payments, &stubVendorRepo{})

	_, err := svc.FindPayment(testCtx, FindPaymentQuery{ID: 100})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}
