package find

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

func newDeliveryTestService(dr *stubDeliveryRepo, pr *stubProjectRepo, cr *stubClientRepo) *Service {
	repos := newTestRepos()
	repos.Deliveries = dr
	repos.Projects = pr
	repos.Clients = cr
	return New(repos)
}

// N06-D01: ID branch ハッピーパス。
func TestService_FindDelivery_ByID_HappyPath(t *testing.T) {
	dr := &stubDeliveryRepo{getByDocIDResult: &boardapi.DeliveryEntity{ID: 300}}
	pr := &stubProjectRepo{
		// reverseMap build: project に Deliveries=[{ID:300}] を持たせる
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Deliveries: []boardapi.DocumentSummary{{ID: 300}}, Client: &boardapi.ClientRef{ID: 5}},
		},
		getResult: &boardapi.ProjectEntity{ID: 7, Client: &boardapi.ClientRef{ID: 5}},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 300})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Delivery.ID != 300 || r.ProjectID != 7 || r.ClientID != 5 {
		t.Errorf("unexpected: %+v", r)
	}
}

// N06-D02: ID branch、reverseMap miss。
func TestService_FindDelivery_ByID_ReverseMapMiss_PartialResult(t *testing.T) {
	dr := &stubDeliveryRepo{getByDocIDResult: &boardapi.DeliveryEntity{ID: 999}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	cr := &stubClientRepo{}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 999})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 0 {
		t.Fatalf("expected partial: %+v", results)
	}
}

// N06-D03: ID branch、Document fetch error 伝播。
func TestService_FindDelivery_ByID_DocumentFetchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	dr := &stubDeliveryRepo{err: fakeErr}
	svc := newDeliveryTestService(dr, &stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 300})
	if !errors.Is(err, fakeErr) {
		t.Errorf("got %v", err)
	}
}

// N06-D04: ID branch、projects.GetByID fail → non-fatal。
func TestService_FindDelivery_ByID_ProjectFetchFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	fakeErr := errors.New("project fetch failed")
	dr := &stubDeliveryRepo{getByDocIDResult: &boardapi.DeliveryEntity{ID: 300}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{{ID: 7, Deliveries: []boardapi.DocumentSummary{{ID: 300}}}},
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, fakeErr
		},
	}
	svc := newDeliveryTestService(dr, pr, &stubClientRepo{})
	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 300})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 7 || results[0].Project != nil {
		t.Fatalf("unexpected: %+v", results)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 || warns[0].Message != "find.FindDelivery: project enrichment failed" {
		t.Errorf("unexpected warns: %+v", warns)
	}
}

// N06-D05: ProjectID branch、複数 Deliveries 全件ループ（**主要テスト**：旧 find/[0] 廃止）。
func TestService_FindDelivery_ByProjectID_MultipleDeliveries_LoopsAll(t *testing.T) {
	docCount := 0
	dr := &stubDeliveryRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.DeliveryEntity, error) {
			docCount++
			return &boardapi.DeliveryEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		getWithGroupResult: &boardapi.ProjectEntity{
			ID: 7,
			Deliveries: []boardapi.DocumentSummary{
				{ID: 300},
				{ID: 301},
			},
			Client: &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (loops all Deliveries), got %d", len(results))
	}
	if results[0].Delivery.ID != 300 || results[1].Delivery.ID != 301 {
		t.Errorf("expected IDs [300,301], got [%d,%d]", results[0].Delivery.ID, results[1].Delivery.ID)
	}
	if docCount != 2 {
		t.Errorf("expected GetByDocumentID called 2 times, got %d", docCount)
	}
	// client 取得は project ごとに 1 回（ループ前 1 回のみ）
	if cr.getCount != 1 {
		t.Errorf("expected clients.GetByID called once per project, got %d", cr.getCount)
	}
}

// N06-D06: ProjectID branch、Deliveries なし → 0 件。
func TestService_FindDelivery_ByProjectID_NoDeliveries_ReturnsEmpty(t *testing.T) {
	pr := &stubProjectRepo{getWithGroupResult: &boardapi.ProjectEntity{ID: 7, Deliveries: nil}}
	svc := newDeliveryTestService(&stubDeliveryRepo{}, pr, &stubClientRepo{})
	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// N06-D07: ProjectID branch、GetByIDWithGroup error 伝播。
func TestService_FindDelivery_ByProjectID_GetWithGroupError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	pr := &stubProjectRepo{getWithGroupErr: fakeErr}
	svc := newDeliveryTestService(&stubDeliveryRepo{}, pr, &stubClientRepo{})
	_, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ProjectID: 7})
	if !errors.Is(err, fakeErr) {
		t.Errorf("got %v", err)
	}
}

// N06-D08: ClientName branch、配列 Deliveries 全件ループ。
func TestService_FindDelivery_ByClientName_FanoutsAcrossDeliveries(t *testing.T) {
	dr := &stubDeliveryRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.DeliveryEntity, error) {
			return &boardapi.DeliveryEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{
				ID: 7,
				Deliveries: []boardapi.DocumentSummary{
					{ID: 300},
					{ID: 301},
				},
				Client: &boardapi.ClientRef{ID: 5},
			},
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}}}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ClientName: "Acme"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// outer loop の c を再利用するため clients.GetByID は呼ばれない
	if cr.getCount != 0 {
		t.Errorf("expected clients.GetByID not called (outer reuse), got %d", cr.getCount)
	}
}

// N06-D09: ProjectName branch、Search filter assert。
func TestService_FindDelivery_ByProjectName_HappyPath(t *testing.T) {
	var captured boardapi.ProjectListOptions
	dr := &stubDeliveryRepo{getByDocIDResult: &boardapi.DeliveryEntity{ID: 300}}
	pr := &stubProjectRepo{
		searchFunc: func(_ context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			captured = filter
			return []boardapi.ProjectEntity{
				{ID: 7, Deliveries: []boardapi.DocumentSummary{{ID: 300}}, Client: &boardapi.ClientRef{ID: 5}},
			}, nil
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ProjectName: "Foo"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if captured.NameCont != "Foo" || captured.ResponseGroup != "delivery" {
		t.Errorf("captured: %+v", captured)
	}
}

// N06-D10: ProjectName + Limit=2 で配列ループの 3 件目に進まないこと。
func TestService_FindDelivery_LimitTwo_StopsAcrossInnerLoop(t *testing.T) {
	count := 0
	dr := &stubDeliveryRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.DeliveryEntity, error) {
			count++
			return &boardapi.DeliveryEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{
				ID: 7,
				Deliveries: []boardapi.DocumentSummary{
					{ID: 201},
					{ID: 202},
				},
			},
			{
				ID: 8,
				Deliveries: []boardapi.DocumentSummary{
					{ID: 203},
				},
			},
		},
	}
	cr := &stubClientRepo{}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{
		ProjectName:    "x",
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if count != 2 {
		t.Errorf("expected GetByDocumentID called 2 times (stopped at limit), got %d", count)
	}
}

// N06-D11: ClientName branch、IsNotFound はスキップして次の Document を試す。
func TestService_FindDelivery_ByClientName_DocumentNotFoundSkipped(t *testing.T) {
	dr := &stubDeliveryRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.DeliveryEntity, error) {
			if id == 300 {
				return nil, fakeNotFoundErr()
			}
			return &boardapi.DeliveryEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{
				ID: 7,
				Deliveries: []boardapi.DocumentSummary{
					{ID: 300},
					{ID: 301},
				},
				Client: &boardapi.ClientRef{ID: 5},
			},
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}}}
	svc := newDeliveryTestService(dr, pr, cr)

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ClientName: "x"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (300 skipped, 301 adopted), got %d", len(results))
	}
	if results[0].Delivery.ID != 301 {
		t.Errorf("expected ID=301, got %d", results[0].Delivery.ID)
	}
}

// --- バリデーション ---

func TestService_FindDelivery_EmptyQuery_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindDelivery(testCtx, FindDeliveryQuery{})
	if err == nil || err.Error() != "at least one field required" {
		t.Errorf("got %v", err)
	}
}

func TestService_FindDelivery_LimitNegative_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 1, FindCommonOpts: FindCommonOpts{Limit: -1}})
	if err == nil || err.Error() != "limit must be >= 0" {
		t.Errorf("got %v", err)
	}
}

func TestService_FindDelivery_PriorityIDOverridesProjectID(t *testing.T) {
	dr := &stubDeliveryRepo{getByDocIDResult: &boardapi.DeliveryEntity{ID: 300}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	svc := newDeliveryTestService(dr, pr, &stubClientRepo{})

	results, err := svc.FindDelivery(testCtx, FindDeliveryQuery{ID: 300, ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if pr.getWithGroupCount != 0 {
		t.Errorf("expected GetByIDWithGroup not called, got %d", pr.getWithGroupCount)
	}
}
