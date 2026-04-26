package find

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

func newOrderTestService(or *stubOrderRepo, pr *stubProjectRepo, cr *stubClientRepo) *Service {
	repos := newTestRepos()
	repos.Orders = or
	repos.Projects = pr
	repos.Clients = cr
	return New(repos)
}

// N06-O01: ID branch ハッピーパス。
func TestService_FindOrder_ByID_HappyPath(t *testing.T) {
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 200}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Order: &boardapi.DocumentSummary{ID: 200}, Client: &boardapi.ClientRef{ID: 5}},
		},
		getResult: &boardapi.ProjectEntity{ID: 7, Client: &boardapi.ClientRef{ID: 5}},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 200})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Order.ID != 200 || r.ProjectID != 7 || r.ClientID != 5 {
		t.Errorf("unexpected result: %+v", r)
	}
	if r.Project == nil || r.Project.ID != 7 {
		t.Errorf("Project=%v", r.Project)
	}
}

// N06-O02: ID branch、reverseMap miss。
func TestService_FindOrder_ByID_ReverseMapMiss_PartialResult(t *testing.T) {
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 999}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	cr := &stubClientRepo{}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 999})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 0 {
		t.Fatalf("expected partial result with ProjectID=0, got %+v", results)
	}
}

// N06-O03: ID branch、Document fetch error 伝播。
func TestService_FindOrder_ByID_DocumentFetchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	or := &stubOrderRepo{err: fakeErr}
	svc := newOrderTestService(or, &stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 200})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected upstream error, got %v", err)
	}
}

// N06-O04: ID branch、projects.GetByID fail → non-fatal。
func TestService_FindOrder_ByID_ProjectFetchFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	fakeErr := errors.New("project fetch failed")
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 200}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{{ID: 7, Order: &boardapi.DocumentSummary{ID: 200}}},
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, fakeErr
		},
	}
	cr := &stubClientRepo{}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 200})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 7 || results[0].Project != nil {
		t.Fatalf("unexpected partial result: %+v", results)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 || warns[0].Message != "find.FindOrder: project enrichment failed" {
		t.Errorf("unexpected warns: %+v", warns)
	}
}

// N06-O05: ProjectID branch ハッピーパス。
func TestService_FindOrder_ByProjectID_HappyPath(t *testing.T) {
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 200}}
	pr := &stubProjectRepo{
		getWithGroupResult: &boardapi.ProjectEntity{
			ID:     7,
			Order:  &boardapi.DocumentSummary{ID: 200},
			Client: &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if pr.getCount != 0 {
		t.Errorf("expected no double-fetch, got getCount=%d", pr.getCount)
	}
}

// N06-O06: ProjectID branch、Order なし → 0 件。
func TestService_FindOrder_ByProjectID_NoOrder_ReturnsEmpty(t *testing.T) {
	pr := &stubProjectRepo{getWithGroupResult: &boardapi.ProjectEntity{ID: 7, Order: nil}}
	svc := newOrderTestService(&stubOrderRepo{}, pr, &stubClientRepo{})
	results, err := svc.FindOrder(testCtx, FindOrderQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// N06-O07: ProjectID branch、GetByIDWithGroup error 伝播。
func TestService_FindOrder_ByProjectID_GetWithGroupError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	pr := &stubProjectRepo{getWithGroupErr: fakeErr}
	svc := newOrderTestService(&stubOrderRepo{}, pr, &stubClientRepo{})
	_, err := svc.FindOrder(testCtx, FindOrderQuery{ProjectID: 7})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected upstream error, got %v", err)
	}
}

// N06-O08: ClientName branch、複数 fanout。
func TestService_FindOrder_ByClientName_FanoutsAcrossClientsAndProjects(t *testing.T) {
	or := &stubOrderRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.OrderEntity, error) {
			return &boardapi.OrderEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchFunc: func(_ context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			switch filter.ClientIDEq {
			case 5:
				return []boardapi.ProjectEntity{{ID: 71, Order: &boardapi.DocumentSummary{ID: 1001}, Client: &boardapi.ClientRef{ID: 5}}}, nil
			case 6:
				return []boardapi.ProjectEntity{{ID: 72, Order: &boardapi.DocumentSummary{ID: 1002}, Client: &boardapi.ClientRef{ID: 6}}}, nil
			}
			return nil, nil
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}, {ID: 6}}}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ClientName: "Acme"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if cr.getCount != 0 {
		t.Errorf("expected clients.GetByID not called (outer loop reuse), got %d", cr.getCount)
	}
}

// N06-O09: ProjectName branch、Search filter assert。
func TestService_FindOrder_ByProjectName_HappyPath(t *testing.T) {
	var captured boardapi.ProjectListOptions
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 200}}
	pr := &stubProjectRepo{
		searchFunc: func(_ context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			captured = filter
			return []boardapi.ProjectEntity{{ID: 7, Order: &boardapi.DocumentSummary{ID: 200}, Client: &boardapi.ClientRef{ID: 5}}}, nil
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newOrderTestService(or, pr, cr)

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ProjectName: "Foo"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if captured.NameCont != "Foo" || captured.ResponseGroup != "order" {
		t.Errorf("captured filter mismatch: %+v", captured)
	}
}

// N06-O10: ClientName + Limit=1。
func TestService_FindOrder_ByClientName_LimitOne(t *testing.T) {
	count := 0
	or := &stubOrderRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.OrderEntity, error) {
			count++
			return &boardapi.OrderEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 71, Order: &boardapi.DocumentSummary{ID: 1001}, Client: &boardapi.ClientRef{ID: 5}},
			{ID: 72, Order: &boardapi.DocumentSummary{ID: 1002}, Client: &boardapi.ClientRef{ID: 5}},
			{ID: 73, Order: &boardapi.DocumentSummary{ID: 1003}, Client: &boardapi.ClientRef{ID: 5}},
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}}}
	svc := newOrderTestService(or, pr, cr)
	results, err := svc.FindOrder(testCtx, FindOrderQuery{ClientName: "x", FindCommonOpts: FindCommonOpts{Limit: 1}})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if count != 1 {
		t.Errorf("expected GetByDocumentID called once, got %d", count)
	}
}

// --- バリデーション ---

func TestService_FindOrder_EmptyQuery_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindOrder(testCtx, FindOrderQuery{})
	if err == nil || err.Error() != "at least one field required" {
		t.Errorf("expected 'at least one field required', got %v", err)
	}
}

func TestService_FindOrder_LimitNegative_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 1, FindCommonOpts: FindCommonOpts{Limit: -1}})
	if err == nil || err.Error() != "limit must be >= 0" {
		t.Errorf("expected 'limit must be >= 0', got %v", err)
	}
}

func TestService_FindOrder_TextOnly_ReturnsEmpty(t *testing.T) {
	or := &stubOrderRepo{}
	pr := &stubProjectRepo{}
	cr := &stubClientRepo{}
	svc := newOrderTestService(or, pr, cr)
	results, err := svc.FindOrder(testCtx, FindOrderQuery{Text: "abc"})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestService_FindOrder_PriorityIDOverridesProjectID(t *testing.T) {
	or := &stubOrderRepo{getByDocIDResult: &boardapi.OrderEntity{ID: 200}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	svc := newOrderTestService(or, pr, &stubClientRepo{})

	results, err := svc.FindOrder(testCtx, FindOrderQuery{ID: 200, ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if pr.getWithGroupCount != 0 {
		t.Errorf("expected GetByIDWithGroup not called, got %d", pr.getWithGroupCount)
	}
}
