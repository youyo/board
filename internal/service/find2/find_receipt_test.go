package find2

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

func newReceiptTestService(rr *stubReceiptRepo, pr *stubProjectRepo, cr *stubClientRepo) *Service {
	repos := newTestRepos()
	repos.Receipts = rr
	repos.Projects = pr
	repos.Clients = cr
	return New(repos)
}

// N06-R01: ID branch ハッピーパス。
func TestService_FindReceipt_ByID_HappyPath(t *testing.T) {
	rr := &stubReceiptRepo{getByDocIDResult: &boardapi.ReceiptEntity{ID: 400}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Receipts: []boardapi.DocumentSummary{{ID: 400}}, Client: &boardapi.ClientRef{ID: 5}},
		},
		getResult: &boardapi.ProjectEntity{ID: 7, Client: &boardapi.ClientRef{ID: 5}},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newReceiptTestService(rr, pr, cr)

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 400})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	r := results[0]
	if r.Receipt.ID != 400 || r.ProjectID != 7 || r.ClientID != 5 {
		t.Errorf("unexpected: %+v", r)
	}
}

// N06-R02: ID branch、reverseMap miss。
func TestService_FindReceipt_ByID_ReverseMapMiss_PartialResult(t *testing.T) {
	rr := &stubReceiptRepo{getByDocIDResult: &boardapi.ReceiptEntity{ID: 999}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	svc := newReceiptTestService(rr, pr, &stubClientRepo{})

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 999})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 0 {
		t.Fatalf("unexpected: %+v", results)
	}
}

// N06-R03: ID branch、Document fetch error 伝播。
func TestService_FindReceipt_ByID_DocumentFetchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	rr := &stubReceiptRepo{err: fakeErr}
	svc := newReceiptTestService(rr, &stubProjectRepo{}, &stubClientRepo{})
	_, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 400})
	if !errors.Is(err, fakeErr) {
		t.Errorf("got %v", err)
	}
}

// N06-R04: ID branch、projects.GetByID fail → non-fatal。
func TestService_FindReceipt_ByID_ProjectFetchFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	fakeErr := errors.New("project fetch failed")
	rr := &stubReceiptRepo{getByDocIDResult: &boardapi.ReceiptEntity{ID: 400}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{{ID: 7, Receipts: []boardapi.DocumentSummary{{ID: 400}}}},
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, fakeErr
		},
	}
	svc := newReceiptTestService(rr, pr, &stubClientRepo{})
	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 400})
	assertNoError(t, err)
	if len(results) != 1 || results[0].ProjectID != 7 || results[0].Project != nil {
		t.Fatalf("unexpected: %+v", results)
	}
	warns := filterWarn(rec.records)
	if len(warns) != 1 || warns[0].Message != "find2.FindReceipt: project enrichment failed" {
		t.Errorf("unexpected warns: %+v", warns)
	}
}

// N06-R05: ProjectID branch、複数 Receipts 全件ループ（**配列対応**）。
func TestService_FindReceipt_ByProjectID_MultipleReceipts_LoopsAll(t *testing.T) {
	docCount := 0
	rr := &stubReceiptRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.ReceiptEntity, error) {
			docCount++
			return &boardapi.ReceiptEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		getWithGroupResult: &boardapi.ProjectEntity{
			ID: 7,
			Receipts: []boardapi.DocumentSummary{
				{ID: 400},
				{ID: 401},
				{ID: 402},
			},
			Client: &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newReceiptTestService(rr, pr, cr)

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 3 {
		t.Fatalf("expected 3 (loops all), got %d", len(results))
	}
	if docCount != 3 {
		t.Errorf("expected GetByDocumentID called 3 times, got %d", docCount)
	}
	if cr.getCount != 1 {
		t.Errorf("expected client lookup once per project, got %d", cr.getCount)
	}
}

// N06-R06: ProjectID branch、Receipts なし → 0 件。
func TestService_FindReceipt_ByProjectID_NoReceipts_ReturnsEmpty(t *testing.T) {
	pr := &stubProjectRepo{getWithGroupResult: &boardapi.ProjectEntity{ID: 7, Receipts: nil}}
	svc := newReceiptTestService(&stubReceiptRepo{}, pr, &stubClientRepo{})
	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// N06-R07: ProjectID branch、GetByIDWithGroup error 伝播。
func TestService_FindReceipt_ByProjectID_GetWithGroupError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	pr := &stubProjectRepo{getWithGroupErr: fakeErr}
	svc := newReceiptTestService(&stubReceiptRepo{}, pr, &stubClientRepo{})
	_, err := svc.FindReceipt(testCtx, FindReceiptQuery{ProjectID: 7})
	if !errors.Is(err, fakeErr) {
		t.Errorf("got %v", err)
	}
}

// N06-R08: ClientName branch、配列 fanout。
func TestService_FindReceipt_ByClientName_FanoutsAcrossReceipts(t *testing.T) {
	rr := &stubReceiptRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.ReceiptEntity, error) {
			return &boardapi.ReceiptEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{
				ID: 7,
				Receipts: []boardapi.DocumentSummary{
					{ID: 400},
					{ID: 401},
				},
				Client: &boardapi.ClientRef{ID: 5},
			},
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}}}
	svc := newReceiptTestService(rr, pr, cr)

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ClientName: "Acme"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if cr.getCount != 0 {
		t.Errorf("expected outer reuse, got %d", cr.getCount)
	}
}

// N06-R09: ProjectName branch、Search filter assert。
func TestService_FindReceipt_ByProjectName_HappyPath(t *testing.T) {
	var captured boardapi.ProjectListOptions
	rr := &stubReceiptRepo{getByDocIDResult: &boardapi.ReceiptEntity{ID: 400}}
	pr := &stubProjectRepo{
		searchFunc: func(_ context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			captured = filter
			return []boardapi.ProjectEntity{
				{ID: 7, Receipts: []boardapi.DocumentSummary{{ID: 400}}, Client: &boardapi.ClientRef{ID: 5}},
			}, nil
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newReceiptTestService(rr, pr, cr)

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ProjectName: "Foo"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if captured.NameCont != "Foo" || captured.ResponseGroup != "receipt" {
		t.Errorf("captured: %+v", captured)
	}
}

// N06-R10: ProjectName + Limit=2。
func TestService_FindReceipt_LimitTwo_StopsAcrossInnerLoop(t *testing.T) {
	count := 0
	rr := &stubReceiptRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.ReceiptEntity, error) {
			count++
			return &boardapi.ReceiptEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Receipts: []boardapi.DocumentSummary{{ID: 401}, {ID: 402}}},
			{ID: 8, Receipts: []boardapi.DocumentSummary{{ID: 403}}},
		},
	}
	svc := newReceiptTestService(rr, pr, &stubClientRepo{})
	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ProjectName: "x", FindCommonOpts: FindCommonOpts{Limit: 2}})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

// N06-R11: ClientName branch、IsNotFound はスキップ。
func TestService_FindReceipt_ByClientName_DocumentNotFoundSkipped(t *testing.T) {
	rr := &stubReceiptRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.ReceiptEntity, error) {
			if id == 400 {
				return nil, fakeNotFoundErr()
			}
			return &boardapi.ReceiptEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{
				ID: 7,
				Receipts: []boardapi.DocumentSummary{
					{ID: 400},
					{ID: 401},
				},
				Client: &boardapi.ClientRef{ID: 5},
			},
		},
	}
	cr := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 5}}}
	svc := newReceiptTestService(rr, pr, cr)

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ClientName: "x"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].Receipt.ID != 401 {
		t.Errorf("expected ID=401, got %d", results[0].Receipt.ID)
	}
}

// --- バリデーション ---

func TestService_FindReceipt_EmptyQuery_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindReceipt(testCtx, FindReceiptQuery{})
	if err == nil || err.Error() != "at least one field required" {
		t.Errorf("got %v", err)
	}
}

func TestService_FindReceipt_LimitNegative_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 1, FindCommonOpts: FindCommonOpts{Limit: -1}})
	if err == nil || err.Error() != "limit must be >= 0" {
		t.Errorf("got %v", err)
	}
}

func TestService_FindReceipt_TextOnly_ReturnsEmpty(t *testing.T) {
	rr := &stubReceiptRepo{}
	pr := &stubProjectRepo{}
	cr := &stubClientRepo{}
	svc := newReceiptTestService(rr, pr, cr)
	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{Text: "abc"})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("got %d", len(results))
	}
}

func TestService_FindReceipt_PriorityIDOverridesProjectID(t *testing.T) {
	rr := &stubReceiptRepo{getByDocIDResult: &boardapi.ReceiptEntity{ID: 400}}
	pr := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{}}
	svc := newReceiptTestService(rr, pr, &stubClientRepo{})

	results, err := svc.FindReceipt(testCtx, FindReceiptQuery{ID: 400, ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if pr.getWithGroupCount != 0 {
		t.Errorf("expected GetByIDWithGroup not called, got %d", pr.getWithGroupCount)
	}
}
