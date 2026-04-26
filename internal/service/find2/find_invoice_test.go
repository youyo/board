package find2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// newInvoiceTestService は FindInvoice テスト用の Service を生成するヘルパー。
func newInvoiceTestService(
	invoices *stubInvoiceRepo,
	clients *stubClientRepo,
	projects *stubProjectRepo,
) *Service {
	r := newTestRepos()
	r.Invoices = invoices
	r.Clients = clients
	r.Projects = projects
	return New(r)
}

// I01: ByID HappyPath（ClientID/ProjectID enrichment 成功）
func TestService_FindInvoice_ByID_HappyPath(t *testing.T) {
	invoices := &stubInvoiceRepo{
		getResult: &boardapi.InvoiceEntity{ID: 100, ClientID: 5, ProjectID: 7, Title: "Inv-100"},
	}
	clients := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5, Name: "C"}}
	projects := &stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 7, Name: "P"}}
	svc := newInvoiceTestService(invoices, clients, projects)

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Invoice.ID != 100 {
		t.Errorf("Invoice.ID=%d, want 100", r.Invoice.ID)
	}
	if r.Client == nil || r.Client.ID != 5 {
		t.Errorf("Client mismatch: %+v", r.Client)
	}
	if r.Project == nil || r.Project.ID != 7 {
		t.Errorf("Project mismatch: %+v", r.Project)
	}
}

// I02: ByID — Project enrichment 失敗 → non-fatal（Project=nil + slog.Warn）
func TestService_FindInvoice_ByID_ProjectEnrichmentFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	invoices := &stubInvoiceRepo{
		getResult: &boardapi.InvoiceEntity{ID: 100, ClientID: 5, ProjectID: 7},
	}
	clients := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	projects := &stubProjectRepo{
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, errors.New("project fetch failed")
		},
	}
	svc := newInvoiceTestService(invoices, clients, projects)

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project != nil {
		t.Errorf("Project should be nil after fail, got %+v", results[0].Project)
	}
	if results[0].Client == nil || results[0].Client.ID != 5 {
		t.Errorf("Client should succeed: %+v", results[0].Client)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn record, got %d", len(warns))
	}
	if !strings.Contains(warns[0].Message, "project enrichment failed") {
		t.Errorf("unexpected slog message: %q", warns[0].Message)
	}
}

// I03: ByID — Statuses post-filter は ID 検索時 skip（旧 find_project.go 踏襲）
func TestService_FindInvoice_ByID_StatusesPostFilterSkipped(t *testing.T) {
	invoices := &stubInvoiceRepo{
		// status=archived だが Statuses=[sent] 指定でも返却される（ID 検索時 skip）
		getResult: &boardapi.InvoiceEntity{ID: 100, Status: "archived"},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ID: 100, Statuses: []string{"sent"}})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (status filter skipped on ID), got %d", len(results))
	}
}

// I04: ByClientID — ClientIDEq が API に渡る、Status="" の場合 StatusEq も ""
func TestService_FindInvoice_ByClientID_DelegatesFilter(t *testing.T) {
	var captured boardapi.InvoiceListOptions
	invoices := &stubInvoiceRepo{
		searchFunc: func(_ context.Context, f boardapi.InvoiceListOptions, _ repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
			captured = f
			return []boardapi.InvoiceEntity{{ID: 1}, {ID: 2}}, nil
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ClientID: 5})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if captured.ClientIDEq != 5 {
		t.Errorf("ClientIDEq=%d, want 5", captured.ClientIDEq)
	}
	if captured.StatusEq != "" {
		t.Errorf("StatusEq=%q, want empty", captured.StatusEq)
	}
}

// I05: ByClientID + Status — ClientIDEq + StatusEq 両方 API 委譲
func TestService_FindInvoice_ByClientIDAndStatus_DelegatesBoth(t *testing.T) {
	var captured boardapi.InvoiceListOptions
	invoices := &stubInvoiceRepo{
		searchFunc: func(_ context.Context, f boardapi.InvoiceListOptions, _ repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
			captured = f
			return []boardapi.InvoiceEntity{{ID: 1}}, nil
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	_, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ClientID: 5, Status: "sent"})
	assertNoError(t, err)
	if captured.ClientIDEq != 5 || captured.StatusEq != "sent" {
		t.Errorf("filter mismatch: %+v", captured)
	}
}

// I06: ByStatus（単独）— Status only は allow、StatusEq が API に渡る
func TestService_FindInvoice_ByStatus_Only_AllowedAndDelegated(t *testing.T) {
	var captured boardapi.InvoiceListOptions
	invoices := &stubInvoiceRepo{
		searchFunc: func(_ context.Context, f boardapi.InvoiceListOptions, _ repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
			captured = f
			return []boardapi.InvoiceEntity{{ID: 1, Status: "sent"}}, nil
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{Status: "sent"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if captured.StatusEq != "sent" {
		t.Errorf("StatusEq=%q, want 'sent'", captured.StatusEq)
	}
	if captured.ClientIDEq != 0 {
		t.Errorf("ClientIDEq should be 0, got %d", captured.ClientIDEq)
	}
}

// I07: ByText — Title マッチ
func TestService_FindInvoice_ByText_MatchesTitle(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{
			{ID: 1, Title: "Acme Invoice"},
			{ID: 2, Title: "Beta Receipt"},
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{Text: "acme"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Invoice.Title != "Acme Invoice" {
		t.Errorf("wrong invoice: %+v", results[0].Invoice)
	}
}

// I08: ByText — Memo マッチ
func TestService_FindInvoice_ByText_MatchesMemo(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{
			{ID: 1, Title: "x", Memo: "important payment"},
			{ID: 2, Title: "y", Memo: "other"},
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{Text: "important"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// I09: ByStatuses (multi) + ClientID — post-filter で 2/3 残る
func TestService_FindInvoice_ByStatuses_PostFilters(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{
			{ID: 1, Status: "sent"},
			{ID: 2, Status: "draft"},
			{ID: 3, Status: "approved"},
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{
		ClientID: 5,
		Statuses: []string{"sent", "approved"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// I10: Limit=2 — 3 件中 2 件で打ち切り（resolveClientAndProject の呼び出しも 2 回）
func TestService_FindInvoice_LimitTwo_StopsEnrichment(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{
			{ID: 1, ClientID: 5},
			{ID: 2, ClientID: 5},
			{ID: 3, ClientID: 5},
		},
	}
	clients := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newInvoiceTestService(invoices, clients, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{
		ClientID:       5,
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if clients.getCount != 2 {
		t.Errorf("expected 2 client GetByID calls (Limit=2), got %d", clients.getCount)
	}
}

// I11: Empty query → "at least one field required"
func TestService_FindInvoice_EmptyQuery_Error(t *testing.T) {
	svc := newInvoiceTestService(&stubInvoiceRepo{}, &stubClientRepo{}, &stubProjectRepo{})

	_, err := svc.FindInvoice(testCtx, FindInvoiceQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// I12: Statuses-only → "Statuses requires one of ..."（D2 reject）
func TestService_FindInvoice_StatusesOnly_RejectedByValidate(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{{ID: 1}},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	_, err := svc.FindInvoice(testCtx, FindInvoiceQuery{Statuses: []string{"sent"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Statuses requires one of") {
		t.Errorf("unexpected error message: %v", err)
	}
	// Search は呼ばれない
	if invoices.searchCount != 0 {
		t.Errorf("Search should not be called on validation error, got %d", invoices.searchCount)
	}
}

// I13: GetByID error → fail-fast 伝播
func TestService_FindInvoice_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("invoice API error")
	invoices := &stubInvoiceRepo{err: fakeErr}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	_, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ID: 100})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// I13b: ByText + Statuses — Text branch で Title マッチした 3 件のうち
// Statuses post-filter で 2 件残る（advisor 指摘の Text+Statuses コンボ穴埋め）
func TestService_FindInvoice_ByTextAndStatuses_PostFiltersTextMatched(t *testing.T) {
	invoices := &stubInvoiceRepo{
		searchResult: []boardapi.InvoiceEntity{
			{ID: 1, Title: "Acme A", Status: "sent"},
			{ID: 2, Title: "Acme B", Status: "draft"},
			{ID: 3, Title: "Acme C", Status: "approved"},
			{ID: 4, Title: "Beta", Status: "sent"}, // Text マッチしない
		},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	results, err := svc.FindInvoice(testCtx, FindInvoiceQuery{
		Text:     "acme",
		Statuses: []string{"sent", "approved"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Text=acme ∩ Statuses={sent,approved}), got %d", len(results))
	}
	for _, r := range results {
		if r.Invoice.Status != "sent" && r.Invoice.Status != "approved" {
			t.Errorf("unexpected status passed through filter: %q", r.Invoice.Status)
		}
		if !strings.Contains(strings.ToLower(r.Invoice.Title), "acme") {
			t.Errorf("unexpected title passed through Text filter: %q", r.Invoice.Title)
		}
	}
}

// I14: PriorityIDOverridesClientID — ID が優先されると Search は呼ばれない
func TestService_FindInvoice_Priority_IDOverridesClientID(t *testing.T) {
	invoices := &stubInvoiceRepo{
		getResult: &boardapi.InvoiceEntity{ID: 100},
	}
	svc := newInvoiceTestService(invoices, &stubClientRepo{}, &stubProjectRepo{})

	_, err := svc.FindInvoice(testCtx, FindInvoiceQuery{ID: 100, ClientID: 5})
	assertNoError(t, err)
	if invoices.searchCount != 0 {
		t.Errorf("Search should not be called when ID is set, got %d", invoices.searchCount)
	}
}
