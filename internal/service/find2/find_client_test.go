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

// newClientTestService は FindClient テスト用の Service を生成するヘルパー。
func newClientTestService(
	clients *stubClientRepo,
	branches *stubClientBranchRepo,
	contacts *stubContactRepo,
) *Service {
	r := newTestRepos()
	r.Clients = clients
	r.ClientBranches = branches
	r.Contacts = contacts
	return New(r)
}

// N04-T01: FindClient by ID — 正常系（branches + contacts 取得）
func TestService_FindClient_ByID_Success(t *testing.T) {
	clients := &stubClientRepo{
		getResult: &boardapi.ClientEntity{ID: 10, Name: "X"},
	}
	branches := &stubClientBranchRepo{
		searchResult: []boardapi.ClientBranchEntity{{ID: 1}},
	}
	contacts := &stubContactRepo{
		searchResult: []boardapi.ContactEntity{{ID: 2}},
	}
	svc := newClientTestService(clients, branches, contacts)

	results, err := svc.FindClient(testCtx, FindClientQuery{ID: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Client.ID != 10 || r.Client.Name != "X" {
		t.Errorf("client mismatch: %+v", r.Client)
	}
	if len(r.Branches) != 1 || r.Branches[0].ID != 1 {
		t.Errorf("branches mismatch: %+v", r.Branches)
	}
	if len(r.Contacts) != 1 || r.Contacts[0].ID != 2 {
		t.Errorf("contacts mismatch: %+v", r.Contacts)
	}
}

// N04-T02: FindClient by Name — NameCont が渡されること
func TestService_FindClient_ByName_DelegatesNameCont(t *testing.T) {
	var capturedOpts boardapi.ClientListOptions
	clients := &stubClientRepo{
		searchFunc: func(_ context.Context, filter boardapi.ClientListOptions, _ repository.ReadOptions) ([]boardapi.ClientEntity, error) {
			capturedOpts = filter
			return []boardapi.ClientEntity{{ID: 1, Name: "Acme"}, {ID: 2, Name: "Acme Inc"}}, nil
		},
	}
	branches := &stubClientBranchRepo{}
	contacts := &stubContactRepo{}
	svc := newClientTestService(clients, branches, contacts)

	results, err := svc.FindClient(testCtx, FindClientQuery{Name: "Acme"})
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

// N04-T03: FindClient by Text — in-process フィルタ
func TestService_FindClient_ByText_FiltersInProcess(t *testing.T) {
	clients := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "Acme Corp"},
			{ID: 2, Name: "BetaCo"},
		},
	}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{Text: "acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Client.Name != "Acme Corp" {
		t.Errorf("wrong client: %s", results[0].Client.Name)
	}
}

// N04-T04: FindClient by Text — CustomNo マッチ
func TestService_FindClient_ByText_MatchesCustomNo(t *testing.T) {
	customNo := "C-001"
	clients := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "SomeCo", CustomNo: &customNo},
			{ID: 2, Name: "OtherCo"},
		},
	}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{Text: "C-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// N04-T05: FindClient by Text — Note マッチ
func TestService_FindClient_ByText_MatchesNote(t *testing.T) {
	note := "urgent customer"
	clients := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "SomeCo", Note: &note},
			{ID: 2, Name: "OtherCo"},
		},
	}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{Text: "urgent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// N04-T06: FindClient — ID が優先され、Name が設定されていても Search は呼ばれない
func TestService_FindClient_PriorityIDOverridesName(t *testing.T) {
	clients := &stubClientRepo{
		getResult: &boardapi.ClientEntity{ID: 10, Name: "X"},
	}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{ID: 10, Name: "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if clients.searchCount != 0 {
		t.Errorf("Search should not be called when ID is set, got %d calls", clients.searchCount)
	}
}

// N04-T07: FindClient — Limit=2 で 3 件中 2 件のみ enrichment される
func TestService_FindClient_LimitTwo_StopsEnrichment(t *testing.T) {
	clients := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "Xone"},
			{ID: 2, Name: "Xtwo"},
			{ID: 3, Name: "Xthree"},
		},
	}
	branchCallCount := 0
	branches := &stubClientBranchRepo{
		searchFunc: func(_ context.Context, _ boardapi.ClientBranchListOptions, _ repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
			branchCallCount++
			return nil, nil
		},
	}
	svc := newClientTestService(clients, branches, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{
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

// N04-T08: FindClient — Limit=0 で全件返却
func TestService_FindClient_LimitZero_NoLimit(t *testing.T) {
	clients := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "Xone"},
			{ID: 2, Name: "Xtwo"},
			{ID: 3, Name: "Xthree"},
		},
	}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	results, err := svc.FindClient(testCtx, FindClientQuery{
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

// N04-T09: FindClient — 空クエリ → error "at least one field required"
func TestService_FindClient_EmptyQuery_Error(t *testing.T) {
	svc := newClientTestService(&stubClientRepo{}, &stubClientBranchRepo{}, &stubContactRepo{})

	_, err := svc.FindClient(testCtx, FindClientQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// N04-T10: FindClient — Limit=-1 → error "limit must be >= 0"
func TestService_FindClient_LimitNegative_Error(t *testing.T) {
	svc := newClientTestService(&stubClientRepo{}, &stubClientBranchRepo{}, &stubContactRepo{})

	_, err := svc.FindClient(testCtx, FindClientQuery{
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

// N04-T21: FindClient — GetByID エラーは致命的（伝播する）
func TestService_FindClient_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("API error")
	clients := &stubClientRepo{err: fakeErr}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	_, err := svc.FindClient(testCtx, FindClientQuery{ID: 10})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N04-T22: FindClient — Search エラーは致命的（伝播する）
func TestService_FindClient_SearchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("search API error")
	clients := &stubClientRepo{searchErr: fakeErr}
	svc := newClientTestService(clients, &stubClientBranchRepo{}, &stubContactRepo{})

	_, err := svc.FindClient(testCtx, FindClientQuery{Name: "x"})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N04-T23: FindClient — branches enrichment 失敗は non-fatal（partial result + err=nil）
func TestService_FindClient_BranchEnrichmentFails_PartialResultWithWarn(t *testing.T) {
	fakeErr := errors.New("branches error")
	clients := &stubClientRepo{
		getResult: &boardapi.ClientEntity{ID: 10, Name: "X"},
	}
	branches := &stubClientBranchRepo{err: fakeErr}
	contacts := &stubContactRepo{
		searchResult: []boardapi.ContactEntity{{ID: 2}},
	}
	svc := newClientTestService(clients, branches, contacts)

	results, err := svc.FindClient(testCtx, FindClientQuery{ID: 10})
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
}

// N04-T24: FindClient — ctx cancel 時に両 enrichment が中断し、non-fatal で Result 返却
func TestService_FindClient_CtxCancel_BothEnrichmentsAbort_NonFatal(t *testing.T) {
	clients := &stubClientRepo{
		getResult: &boardapi.ClientEntity{ID: 10, Name: "X"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即 cancel

	branches := &stubClientBranchRepo{
		searchFunc: func(ctx context.Context, _ boardapi.ClientBranchListOptions, _ repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	contacts := &stubContactRepo{
		searchFunc: func(ctx context.Context, _ boardapi.ContactListOptions, _ repository.ReadOptions) ([]boardapi.ContactEntity, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	svc := newClientTestService(clients, branches, contacts)

	done := make(chan struct{})
	var results []ClientResult
	var err error
	go func() {
		results, err = svc.FindClient(ctx, FindClientQuery{ID: 10})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("FindClient did not return within 100ms after ctx cancel")
	}

	if err != nil {
		t.Errorf("expected nil error (non-fatal ctx cancel), got: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Branches != nil {
		t.Errorf("expected nil Branches, got %+v", results[0].Branches)
	}
	if results[0].Contacts != nil {
		t.Errorf("expected nil Contacts, got %+v", results[0].Contacts)
	}
}

// N04-T25: FindClient — branches と contacts が並列で開始することを handshake チャネルで確認
func TestService_FindClient_BothEnrichmentsStartInParallel_Handshake(t *testing.T) {
	clients := &stubClientRepo{
		getResult: &boardapi.ClientEntity{ID: 10, Name: "X"},
	}

	ch := make(chan struct{}, 2)

	branches := &stubClientBranchRepo{
		searchFunc: func(ctx context.Context, _ boardapi.ClientBranchListOptions, _ repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
			ch <- struct{}{}
			select {
			case <-time.After(50 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return []boardapi.ClientBranchEntity{{ID: 1}}, nil
		},
	}
	contacts := &stubContactRepo{
		searchFunc: func(ctx context.Context, _ boardapi.ContactListOptions, _ repository.ReadOptions) ([]boardapi.ContactEntity, error) {
			ch <- struct{}{}
			select {
			case <-time.After(50 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return []boardapi.ContactEntity{{ID: 2}}, nil
		},
	}
	svc := newClientTestService(clients, branches, contacts)

	go func() {
		_, _ = svc.FindClient(testCtx, FindClientQuery{ID: 10})
	}()

	// 両 stub が 20ms 以内に handshake シグナルを送信することを確認（並列開始の決定論的確認）
	ctxAssert, cancelAssert := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelAssert()
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
			// OK: stub が起動した
		case <-ctxAssert.Done():
			t.Fatalf("only %d stubs started within 20ms (expected parallel execution)", i)
		}
	}
}
