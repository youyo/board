package find2

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// --- ヘルパー ---

func newProjectTestService(pr *stubProjectRepo, cr *stubClientRepo) *Service {
	repos := newTestRepos()
	repos.Projects = pr
	repos.Clients = cr
	return New(repos)
}

// --- 正常系 T01-T08 ---

// N05-T01: ID 指定で 1 件取得、Client enrichment あり
func TestService_FindProject_ByID_Success(t *testing.T) {
	clientEntity := &boardapi.ClientEntity{ID: 5, Name: "C"}
	pr := &stubProjectRepo{
		getResult: &boardapi.ProjectEntity{
			ID:     10,
			Name:   "P",
			Client: &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{getResult: clientEntity}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project.ID != 10 {
		t.Errorf("expected Project.ID=10, got %d", results[0].Project.ID)
	}
	if results[0].Client == nil || results[0].Client.ID != 5 {
		t.Errorf("expected Client.ID=5, got %v", results[0].Client)
	}
}

// N05-T02: ID 指定、Client が nil → enrichment スキップ（getCount==0）
func TestService_FindProject_ByID_ClientNil_NoEnrichment(t *testing.T) {
	pr := &stubProjectRepo{
		getResult: &boardapi.ProjectEntity{ID: 10, Name: "P", Client: nil},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Client != nil {
		t.Errorf("expected Client=nil, got %v", results[0].Client)
	}
	if cr.getCount != 0 {
		t.Errorf("expected getCount=0, got %d", cr.getCount)
	}
}

// N05-T03: ClientID 指定で Search に ClientIDEq が渡されることを assert
func TestService_FindProject_ByClientID_DelegatesClientIDEq(t *testing.T) {
	var capturedFilter boardapi.ProjectListOptions
	pr := &stubProjectRepo{
		searchFunc: func(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			capturedFilter = filter
			return []boardapi.ProjectEntity{
				{ID: 1, Name: "P1", Client: &boardapi.ClientRef{ID: 5}},
				{ID: 2, Name: "P2", Client: &boardapi.ClientRef{ID: 5}},
			}, nil
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ClientID: 5})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if capturedFilter.ClientIDEq != 5 {
		t.Errorf("expected ClientIDEq=5, got %d", capturedFilter.ClientIDEq)
	}
}

// N05-T04: Name 指定で Search に NameCont が渡されることを assert
func TestService_FindProject_ByName_DelegatesNameCont(t *testing.T) {
	var capturedFilter boardapi.ProjectListOptions
	pr := &stubProjectRepo{
		searchFunc: func(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			capturedFilter = filter
			return []boardapi.ProjectEntity{{ID: 1, Name: "Acme Project"}}, nil
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Name: "Acme"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if capturedFilter.NameCont != "Acme" {
		t.Errorf("expected NameCont=Acme, got %q", capturedFilter.NameCont)
	}
}

// N05-T05: Text 指定、Name/ManagementNo のいずれかにマッチする件を返す
func TestService_FindProject_ByText_FiltersInProcess(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "Alpha P"},
			{ID: 2, Name: "Beta"},
			{ID: 3, Name: "Gamma", ManagementNo: strPtr("ALPHA-1")},
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Text: "alpha"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// N05-T06: Text 指定、InHouseMemo にマッチ
func TestService_FindProject_ByText_MatchesInHouseMemo(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", InHouseMemo: strPtr("urgent customer")},
			{ID: 2, Name: "P2"},
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Text: "urgent"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project.ID != 1 {
		t.Errorf("expected ID=1, got %d", results[0].Project.ID)
	}
}

// N05-T07: ID 優先 — ClientID/Name があっても Search は呼ばれない
func TestService_FindProject_PriorityIDOverridesOthers(t *testing.T) {
	pr := &stubProjectRepo{
		getResult: &boardapi.ProjectEntity{ID: 10},
		searchFunc: func(_ context.Context, _ boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			return nil, errors.New("Search must not be called")
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10, ClientID: 5, Name: "X"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// N05-T08: Limit=2 で enrichment が 2 件目で停止（3 件目の getCount が増えない）
func TestService_FindProject_LimitTwo_StopsEnrichment(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", Client: &boardapi.ClientRef{ID: 1}},
			{ID: 2, Name: "P2", Client: &boardapi.ClientRef{ID: 2}},
			{ID: 3, Name: "P3", Client: &boardapi.ClientRef{ID: 3}},
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 1}}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{
		Name:           "x",
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if cr.getCount != 2 {
		t.Errorf("expected getCount=2, got %d", cr.getCount)
	}
}

// --- Status post-filter ケース T09-T13 ---

// N05-T09: Status で OrderStatusName をフィルタ
func TestService_FindProject_StatusFiltersByOrderStatusName(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", OrderStatusName: "見積中(中)"},
			{ID: 2, Name: "P2", OrderStatusName: "完了"},
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x", Status: "見積中(中)"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project.ID != 1 {
		t.Errorf("expected ID=1, got %d", results[0].Project.ID)
	}
}

// N05-T10: Status で DeliveryStatusName をフィルタ
func TestService_FindProject_StatusFiltersByDeliveryStatusName(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", DeliveryStatusName: "未着手"},
			{ID: 2, Name: "P2", DeliveryStatusName: "納品済"},
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x", Status: "未着手"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project.ID != 1 {
		t.Errorf("expected ID=1, got %d", results[0].Project.ID)
	}
}

// N05-T11: Status="完了"、OrderStatusName または DeliveryStatusName のどちらかが一致すれば採用（OR）
func TestService_FindProject_StatusOR_OrderOrDelivery(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", OrderStatusName: "完了", DeliveryStatusName: "未着手"},  // OrderStatus 一致 → 採用
			{ID: 2, Name: "P2", OrderStatusName: "見積中", DeliveryStatusName: "完了"},  // DeliveryStatus 一致 → 採用
			{ID: 3, Name: "P3", OrderStatusName: "見積中", DeliveryStatusName: "未着手"}, // どちらも不一致 → 除外
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x", Status: "完了"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// N05-T12: Statuses=["完了","納品済"] で複数ステータスにマッチ
func TestService_FindProject_Statuses_MultipleMatch(t *testing.T) {
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 1, Name: "P1", OrderStatusName: "完了"},
			{ID: 2, Name: "P2", DeliveryStatusName: "納品済"},
			{ID: 3, Name: "P3", OrderStatusName: "見積中"},
		},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{
		Name:     "x",
		Statuses: []string{"完了", "納品済"},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// N05-T13: Status-only（他フィールドゼロ）は validate で reject される
func TestService_FindProject_StatusOnly_RejectedByValidate(t *testing.T) {
	pr := &stubProjectRepo{}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	_, err := svc.FindProject(testCtx, FindProjectQuery{Status: "完了"})
	assertError(t, err)
	if !strings.Contains(err.Error(), "Status/Statuses requires at least one of") {
		t.Errorf("expected Status/Statuses-only error, got: %v", err)
	}
}

// --- 境界値ケース T14-T19 ---

// N05-T14: 空クエリは error
func TestService_FindProject_EmptyQuery_Error(t *testing.T) {
	svc := newProjectTestService(&stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindProject(testCtx, FindProjectQuery{})
	assertError(t, err)
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// N05-T15: Limit < 0 は error
func TestService_FindProject_LimitNegative_Error(t *testing.T) {
	svc := newProjectTestService(&stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindProject(testCtx, FindProjectQuery{
		ID:             10,
		FindCommonOpts: FindCommonOpts{Limit: -1},
	})
	assertError(t, err)
	if !strings.Contains(err.Error(), "limit must be >= 0") {
		t.Errorf("unexpected error: %v", err)
	}
}

// N05-T16: Status と Statuses 両方セットは error
func TestService_FindProject_StatusAndStatusesBothSet_Error(t *testing.T) {
	svc := newProjectTestService(&stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindProject(testCtx, FindProjectQuery{
		Name:     "x",
		Status:   "A",
		Statuses: []string{"B"},
	})
	assertError(t, err)
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

// N05-T17: Statuses 10 件はバリデーション通過
func TestService_FindProject_StatusesExactlyTen_Allowed(t *testing.T) {
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	statuses := make([]string, 10)
	for i := range statuses {
		statuses[i] = "s"
	}
	_, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x", Statuses: statuses})
	assertNoError(t, err)
}

// N05-T18: Statuses 11 件は error
func TestService_FindProject_StatusesEleven_Rejected(t *testing.T) {
	svc := newProjectTestService(&stubProjectRepo{}, &stubClientRepo{})
	statuses := make([]string, 11)
	for i := range statuses {
		statuses[i] = "s"
	}
	_, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x", Statuses: statuses})
	assertError(t, err)
	if !strings.Contains(err.Error(), "at most 10 statuses") {
		t.Errorf("unexpected error: %v", err)
	}
}

// N05-T19: ID 検索時は Status post-filter をスキップ（Status="不一致" でも 1 件返却）
func TestService_FindProject_IDWithStatus_StatusFilterSkipped(t *testing.T) {
	pr := &stubProjectRepo{
		getResult: &boardapi.ProjectEntity{ID: 10, OrderStatusName: "見積中"},
	}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10, Status: "不一致"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (ID search skips status filter), got %d", len(results))
	}
}

// --- 異常系ケース T20-T22 ---

// N05-T20: GetByID failure は致命的エラーとして伝播
func TestService_FindProject_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("db error")
	pr := &stubProjectRepo{err: fakeErr}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	_, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10})
	assertError(t, err)
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N05-T21: Search failure は致命的エラーとして伝播
func TestService_FindProject_SearchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("search error")
	pr := &stubProjectRepo{err: fakeErr}
	cr := &stubClientRepo{}
	svc := newProjectTestService(pr, cr)

	_, err := svc.FindProject(testCtx, FindProjectQuery{Name: "x"})
	assertError(t, err)
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// N05-T22: client enrichment 失敗は non-fatal — 1 件返却 + slog.Warn 1 回
func TestService_FindProject_ClientEnrichmentFails_NonFatal_LogsWarn(t *testing.T) {
	h := withRecordedSlog(t)
	fakeErr := errors.New("client fetch error")

	pr := &stubProjectRepo{
		getResult: &boardapi.ProjectEntity{
			ID:     10,
			Client: &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{err: fakeErr}
	svc := newProjectTestService(pr, cr)

	results, err := svc.FindProject(testCtx, FindProjectQuery{ID: 10})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Client != nil {
		t.Errorf("expected Client=nil, got %v", results[0].Client)
	}

	// slog.Warn が 1 回記録されていることを確認
	warnCount := 0
	for _, r := range h.records {
		if r.Level == slog.LevelWarn {
			warnCount++
		}
	}
	if warnCount != 1 {
		t.Errorf("expected 1 slog.Warn record, got %d", warnCount)
	}
}
