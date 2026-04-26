package find2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// newVendorTestService は FindVendor テスト用の Service を生成するヘルパー。
func newVendorTestService(
	vendors *stubVendorRepo,
	branches *stubVendorBranchRepo,
	contacts *stubVendorContactRepo,
) *Service {
	r := newTestRepos()
	r.Vendors = vendors
	r.VendorBranches = branches
	r.VendorContacts = contacts
	return New(r)
}

// N04-T11: FindVendor by ID — 正常系（branches + contacts 取得）
func TestService_FindVendor_ByID_Success(t *testing.T) {
	vendors := &stubVendorRepo{
		getResult: &boardapi.VendorEntity{ID: 10, Name: "VendorX"},
	}
	branches := &stubVendorBranchRepo{
		searchResult: []boardapi.VendorBranchEntity{{ID: 1}},
	}
	contacts := &stubVendorContactRepo{
		searchResult: []boardapi.VendorContactEntity{{ID: 2}},
	}
	svc := newVendorTestService(vendors, branches, contacts)

	results, err := svc.FindVendor(testCtx, FindVendorQuery{ID: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Vendor.ID != 10 || r.Vendor.Name != "VendorX" {
		t.Errorf("vendor mismatch: %+v", r.Vendor)
	}
	if len(r.Branches) != 1 || r.Branches[0].ID != 1 {
		t.Errorf("branches mismatch: %+v", r.Branches)
	}
	if len(r.Contacts) != 1 || r.Contacts[0].ID != 2 {
		t.Errorf("contacts mismatch: %+v", r.Contacts)
	}
}

// N04-T12: FindVendor by Name — NameCont が渡されること
func TestService_FindVendor_ByName_DelegatesNameCont(t *testing.T) {
	var capturedOpts boardapi.VendorListOptions
	vendors := &stubVendorRepo{
		searchFunc: func(_ context.Context, filter boardapi.VendorListOptions, _ repository.ReadOptions) ([]boardapi.VendorEntity, error) {
			capturedOpts = filter
			return []boardapi.VendorEntity{{ID: 1, Name: "Acme Vendor"}, {ID: 2, Name: "Acme Supply"}}, nil
		},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{Name: "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if capturedOpts.NameCont != "Acme" {
		t.Errorf("NameCont not set: %+v", capturedOpts)
	}
}

// N04-T13: FindVendor by Text — in-process フィルタ（Name マッチ）
func TestService_FindVendor_ByText_FiltersInProcess(t *testing.T) {
	vendors := &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 1, Name: "Acme Vendor"},
			{ID: 2, Name: "BetaSupply"},
		},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{Text: "acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Vendor.Name != "Acme Vendor" {
		t.Errorf("wrong vendor: %s", results[0].Vendor.Name)
	}
}

// N04-T14: FindVendor by Text — Code マッチ（非ポインタ string）
func TestService_FindVendor_ByText_MatchesCode(t *testing.T) {
	vendors := &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 1, Name: "SomeCo", Code: "V-001"},
			{ID: 2, Name: "OtherCo", Code: "V-999"},
		},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{Text: "V-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// N04-T15: FindVendor by Text — Memo マッチ（非ポインタ string）
func TestService_FindVendor_ByText_MatchesMemo(t *testing.T) {
	vendors := &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 1, Name: "SomeCo", Memo: "important supplier"},
			{ID: 2, Name: "OtherCo", Memo: ""},
		},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{Text: "important"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// N04-T16: FindVendor — ID が優先され、Name が設定されていても Search は呼ばれない
func TestService_FindVendor_PriorityIDOverridesName(t *testing.T) {
	vendors := &stubVendorRepo{
		getResult: &boardapi.VendorEntity{ID: 10, Name: "VendorX"},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{ID: 10, Name: "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if vendors.searchCount != 0 {
		t.Errorf("Search should not be called when ID is set, got %d calls", vendors.searchCount)
	}
}

// N04-T17: FindVendor — Limit=2 で 3 件中 2 件のみ enrichment される
func TestService_FindVendor_LimitTwo_StopsEnrichment(t *testing.T) {
	vendors := &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 1, Name: "Xone"},
			{ID: 2, Name: "Xtwo"},
			{ID: 3, Name: "Xthree"},
		},
	}
	branchCallCount := 0
	branches := &stubVendorBranchRepo{
		searchFunc: func(_ context.Context, _ boardapi.VendorBranchListOptions, _ repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
			branchCallCount++
			return nil, nil
		},
	}
	svc := newVendorTestService(vendors, branches, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{
		Text:           "x",
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if branchCallCount != 2 {
		t.Errorf("expected 2 branch Search calls (Limit=2), got %d", branchCallCount)
	}
}

// N04-T18: FindVendor — Limit=0 で全件返却
func TestService_FindVendor_LimitZero_NoLimit(t *testing.T) {
	vendors := &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 1, Name: "Xone"},
			{ID: 2, Name: "Xtwo"},
			{ID: 3, Name: "Xthree"},
		},
	}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	results, err := svc.FindVendor(testCtx, FindVendorQuery{
		Text:           "x",
		FindCommonOpts: FindCommonOpts{Limit: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

// N04-T19: FindVendor — 空クエリ → error "at least one field required"
func TestService_FindVendor_EmptyQuery_Error(t *testing.T) {
	svc := newVendorTestService(&stubVendorRepo{}, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	_, err := svc.FindVendor(testCtx, FindVendorQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// N04-T20: FindVendor — Limit=-1 → error "limit must be >= 0"
func TestService_FindVendor_LimitNegative_Error(t *testing.T) {
	svc := newVendorTestService(&stubVendorRepo{}, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	_, err := svc.FindVendor(testCtx, FindVendorQuery{
		ID:             10,
		FindCommonOpts: FindCommonOpts{Limit: -1},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "limit must be >= 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// N04-T21v: FindVendor — GetByID エラーは致命的（伝播する）
func TestService_FindVendor_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("vendor API error")
	vendors := &stubVendorRepo{err: fakeErr}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	_, err := svc.FindVendor(testCtx, FindVendorQuery{ID: 10})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N04-T22v: FindVendor — Search エラーは致命的（伝播する）
func TestService_FindVendor_SearchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("vendor search error")
	vendors := &stubVendorRepo{searchErr: fakeErr}
	svc := newVendorTestService(vendors, &stubVendorBranchRepo{}, &stubVendorContactRepo{})

	_, err := svc.FindVendor(testCtx, FindVendorQuery{Name: "x"})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N04-T26: FindVendor — branches enrichment 失敗は non-fatal（partial result + err=nil）
// PayeeIDEq フィルタが正しく使われていることも検証
func TestService_FindVendor_BranchEnrichmentFails_PartialResultWithWarn(t *testing.T) {
	fakeErr := errors.New("vendor branches error")
	vendors := &stubVendorRepo{
		getResult: &boardapi.VendorEntity{ID: 10, Name: "VendorX"},
	}

	// PayeeIDEq が vendor.ID に設定されていることを確認
	var capturedBranchOpts boardapi.VendorBranchListOptions
	branches := &stubVendorBranchRepo{
		searchFunc: func(_ context.Context, filter boardapi.VendorBranchListOptions, _ repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
			capturedBranchOpts = filter
			return nil, fakeErr
		},
	}
	contacts := &stubVendorContactRepo{
		searchResult: []boardapi.VendorContactEntity{{ID: 2}},
	}
	svc := newVendorTestService(vendors, branches, contacts)

	results, err := svc.FindVendor(testCtx, FindVendorQuery{ID: 10})
	if err != nil {
		t.Fatalf("expected nil error (non-fatal), got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Branches != nil {
		t.Errorf("expected nil Branches on failure, got %+v", results[0].Branches)
	}
	if len(results[0].Contacts) != 1 || results[0].Contacts[0].ID != 2 {
		t.Errorf("contacts should succeed: %+v", results[0].Contacts)
	}
	// PayeeIDEq が正しく設定されていることを確認（リスク R2 の検証）
	if capturedBranchOpts.PayeeIDEq != 10 {
		t.Errorf("expected PayeeIDEq=10 (vendor.ID), got %d", capturedBranchOpts.PayeeIDEq)
	}

	// ctx cancel テスト（T24 の Vendor 版）
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vendorsCtx := &stubVendorRepo{
		getResult: &boardapi.VendorEntity{ID: 20, Name: "VendorY"},
	}
	branchesCtx := &stubVendorBranchRepo{
		searchFunc: func(ctx context.Context, _ boardapi.VendorBranchListOptions, _ repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	contactsCtx := &stubVendorContactRepo{
		searchFunc: func(ctx context.Context, _ boardapi.VendorContactListOptions, _ repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	svcCtx := newVendorTestService(vendorsCtx, branchesCtx, contactsCtx)

	done := make(chan struct{})
	var ctxResults []VendorResult
	var ctxErr error
	go func() {
		ctxResults, ctxErr = svcCtx.FindVendor(ctx, FindVendorQuery{ID: 20})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("FindVendor did not return within 100ms after ctx cancel")
	}
	if ctxErr != nil {
		t.Errorf("expected nil error (non-fatal ctx cancel), got: %v", ctxErr)
	}
	if len(ctxResults) != 1 {
		t.Errorf("expected 1 result, got %d", len(ctxResults))
	}
}
