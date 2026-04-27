package find

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// --- ヘルパー（estimate） ---

// newEstimateTestService は Estimate 検索向けに必要な stub を差し込んだ Service を返す。
func newEstimateTestService(
	er *stubEstimateRepo,
	pr *stubProjectRepo,
	cr *stubClientRepo,
) *Service {
	repos := newTestRepos()
	repos.Estimates = er
	repos.Projects = pr
	repos.Clients = cr
	return New(repos)
}

// fakeNotFoundErr は IsNotFound() が true を返す擬似エラー。
func fakeNotFoundErr() error {
	return &boardapi.APIError{Code: boardapi.APIErrorNotFound, Message: "not found"}
}

// --- 正常系・主要ブランチ ---

// N06-E01: ID branch ハッピーパス。reverseMap hit + project + client enrichment 全成功。
func TestService_FindEstimate_ByID_HappyPath(t *testing.T) {
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 100}}
	pr := &stubProjectRepo{
		// reverseMapper.build 用の Search 結果
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Estimate: &boardapi.DocumentSummary{ID: 100}, Client: &boardapi.ClientRef{ID: 5}},
		},
		// projects.GetByID（ID branch 3 hop の 3 段目）
		getResult: &boardapi.ProjectEntity{ID: 7, Client: &boardapi.ClientRef{ID: 5}},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5, Name: "C"}}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Estimate.ID != 100 {
		t.Errorf("Estimate.ID=%d, want 100", r.Estimate.ID)
	}
	if r.ProjectID != 7 {
		t.Errorf("ProjectID=%d, want 7", r.ProjectID)
	}
	if r.ClientID != 5 {
		t.Errorf("ClientID=%d, want 5", r.ClientID)
	}
	if r.Project == nil || r.Project.ID != 7 {
		t.Errorf("Project=%v, want ID=7", r.Project)
	}
	if r.Client == nil || r.Client.ID != 5 {
		t.Errorf("Client=%v, want ID=5", r.Client)
	}
}

// N06-E02: ID branch、reverseMap miss で部分結果（Estimate のみ、ProjectID=0）。
func TestService_FindEstimate_ByID_ReverseMapMiss_PartialResult(t *testing.T) {
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 999}}
	pr := &stubProjectRepo{
		// reverseMap build 用に空リストを返す → Lookup miss
		searchResult: []boardapi.ProjectEntity{},
	}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 999})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ProjectID != 0 {
		t.Errorf("ProjectID=%d, want 0 (miss)", results[0].ProjectID)
	}
	if results[0].Project != nil {
		t.Errorf("Project should be nil, got %v", results[0].Project)
	}
	if pr.getCount != 0 {
		t.Errorf("expected projects.GetByID not called on miss, got %d", pr.getCount)
	}
	if cr.getCount != 0 {
		t.Errorf("expected clients.GetByID not called on miss, got %d", cr.getCount)
	}
}

// N06-E03: ID branch、Document fetch 失敗は fail-fast（呼出元へ伝播）。
func TestService_FindEstimate_ByID_DocumentFetchError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream failure")
	er := &stubEstimateRepo{err: fakeErr}
	pr := &stubProjectRepo{}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 100})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected upstream error, got %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on error, got %v", results)
	}
}

// N06-E04: ID branch、reverseMap hit するが projects.GetByID で失敗 → non-fatal、ProjectID 既知の部分結果。
func TestService_FindEstimate_ByID_ProjectFetchFails_NonFatal(t *testing.T) {
	rec := withRecordedSlog(t)
	fakeErr := errors.New("project fetch failed")
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 100}}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 7, Estimate: &boardapi.DocumentSummary{ID: 100}},
		},
		// GetByID は err
		getFunc: func(_ context.Context) (*boardapi.ProjectEntity, error) {
			return nil, fakeErr
		},
	}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 100})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ProjectID != 7 {
		t.Errorf("ProjectID=%d, want 7", results[0].ProjectID)
	}
	if results[0].Project != nil {
		t.Errorf("Project should be nil after fail, got %v", results[0].Project)
	}
	// slog.Warn 1 回観測（project enrichment failed）。reverseMapper の Info はカウント外。
	warns := filterWarn(rec.records)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warn record, got %d (all=%d)", len(warns), len(rec.records))
	}
	if warns[0].Message != "find.FindEstimate: project enrichment failed" {
		t.Errorf("unexpected slog message: %q", warns[0].Message)
	}
}

// N06-E05: ProjectID branch ハッピーパス。GetByIDWithGroup → estimates.GetByDocumentID → enrichment。
func TestService_FindEstimate_ByProjectID_HappyPath(t *testing.T) {
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 100}}
	pr := &stubProjectRepo{
		getWithGroupResult: &boardapi.ProjectEntity{
			ID:       7,
			Estimate: &boardapi.DocumentSummary{ID: 100},
			Client:   &boardapi.ClientRef{ID: 5},
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ProjectID != 7 {
		t.Errorf("ProjectID=%d, want 7", results[0].ProjectID)
	}
	if results[0].ClientID != 5 {
		t.Errorf("ClientID=%d, want 5", results[0].ClientID)
	}
	if results[0].Project == nil || results[0].Project.ID != 7 {
		t.Errorf("Project=%v, want ID=7", results[0].Project)
	}
	// 二重 fetch なし: projects.GetByID は呼ばれない（getCount==0）。
	if pr.getCount != 0 {
		t.Errorf("expected projects.GetByID not called (no double-fetch), got %d", pr.getCount)
	}
}

// N06-E06: ProjectID branch、project に Estimate なし → 0 件。
func TestService_FindEstimate_ByProjectID_NoEstimate_ReturnsEmpty(t *testing.T) {
	er := &stubEstimateRepo{}
	pr := &stubProjectRepo{
		getWithGroupResult: &boardapi.ProjectEntity{ID: 7, Estimate: nil},
	}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// N06-E07: ProjectID branch、GetByIDWithGroup error → 伝播。
func TestService_FindEstimate_ByProjectID_GetWithGroupError_Bubbles(t *testing.T) {
	fakeErr := errors.New("upstream")
	er := &stubEstimateRepo{}
	pr := &stubProjectRepo{getWithGroupErr: fakeErr}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	_, err := svc.FindEstimate(testCtx, FindEstimateQuery{ProjectID: 7})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected upstream error, got %v", err)
	}
}

// N06-E08: ClientName branch、複数 client × 複数 project の fanout。
func TestService_FindEstimate_ByClientName_FanoutsAcrossClientsAndProjects(t *testing.T) {
	er := &stubEstimateRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.EstimateEntity, error) {
			return &boardapi.EstimateEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchFunc: func(ctx context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			// ClientIDEq に応じて異なる project を返す
			switch filter.ClientIDEq {
			case 5:
				return []boardapi.ProjectEntity{{ID: 71, Estimate: &boardapi.DocumentSummary{ID: 1001}, Client: &boardapi.ClientRef{ID: 5}}}, nil
			case 6:
				return []boardapi.ProjectEntity{{ID: 72, Estimate: &boardapi.DocumentSummary{ID: 1002}, Client: &boardapi.ClientRef{ID: 6}}}, nil
			}
			return nil, nil
		},
	}
	cr := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 5, Name: "Acme A"},
			{ID: 6, Name: "Acme B"},
		},
	}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ClientName: "Acme"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ClientID != 5 || results[1].ClientID != 6 {
		t.Errorf("expected ClientID=[5,6], got [%d,%d]", results[0].ClientID, results[1].ClientID)
	}
	// outer loop の c を再利用するため clients.GetByID は呼ばれない
	if cr.getCount != 0 {
		t.Errorf("expected clients.GetByID not called (outer loop reuse), got %d", cr.getCount)
	}
}

// N06-E09: ProjectName branch、Search filter (NameCont/ResponseGroup) を assert。
func TestService_FindEstimate_ByProjectName_HappyPath(t *testing.T) {
	var captured boardapi.ProjectListOptions
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 100}}
	pr := &stubProjectRepo{
		searchFunc: func(_ context.Context, filter boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			captured = filter
			return []boardapi.ProjectEntity{
				{ID: 7, Estimate: &boardapi.DocumentSummary{ID: 100}, Client: &boardapi.ClientRef{ID: 5}},
			}, nil
		},
	}
	cr := &stubClientRepo{getResult: &boardapi.ClientEntity{ID: 5}}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ProjectName: "Foo"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if captured.NameCont != "Foo" {
		t.Errorf("NameCont=%q, want Foo", captured.NameCont)
	}
	if captured.ResponseGroup != "estimate" {
		t.Errorf("ResponseGroup=%q, want estimate", captured.ResponseGroup)
	}
}

// N06-E10: ClientName branch + Limit=1。最初の hit で打ち切り。
func TestService_FindEstimate_ByClientName_LimitOne(t *testing.T) {
	estimateCount := 0
	er := &stubEstimateRepo{
		getByDocIDFunc: func(_ context.Context, id int) (*boardapi.EstimateEntity, error) {
			estimateCount++
			return &boardapi.EstimateEntity{ID: id}, nil
		},
	}
	pr := &stubProjectRepo{
		searchResult: []boardapi.ProjectEntity{
			{ID: 71, Estimate: &boardapi.DocumentSummary{ID: 1001}, Client: &boardapi.ClientRef{ID: 5}},
			{ID: 72, Estimate: &boardapi.DocumentSummary{ID: 1002}, Client: &boardapi.ClientRef{ID: 5}},
			{ID: 73, Estimate: &boardapi.DocumentSummary{ID: 1003}, Client: &boardapi.ClientRef{ID: 5}},
		},
	}
	cr := &stubClientRepo{
		searchResult: []boardapi.ClientEntity{{ID: 5}},
	}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ClientName: "x", FindCommonOpts: FindCommonOpts{Limit: 1}})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if estimateCount != 1 {
		t.Errorf("expected estimates.GetByDocumentID called 1 time, got %d", estimateCount)
	}
}

// --- バリデーション ---

// N06-V01-E: 空 Query → error。
func TestService_FindEstimate_EmptyQuery_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindEstimate(testCtx, FindEstimateQuery{})
	if err == nil || err.Error() != "at least one field required" {
		t.Errorf("expected 'at least one field required', got %v", err)
	}
}

// N06-V02-E: Limit<0 → error。
func TestService_FindEstimate_LimitNegative_Error(t *testing.T) {
	svc := New(newTestRepos())
	_, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 1, FindCommonOpts: FindCommonOpts{Limit: -1}})
	if err == nil || err.Error() != "limit must be >= 0" {
		t.Errorf("expected 'limit must be >= 0', got %v", err)
	}
}

// N06-V04-E: ID > ProjectID 優先順位。ID branch のみ走り、getWithGroupCount==0。
func TestService_FindEstimate_PriorityIDOverridesProjectID(t *testing.T) {
	er := &stubEstimateRepo{getByDocIDResult: &boardapi.EstimateEntity{ID: 100}}
	pr := &stubProjectRepo{
		// reverseMap miss にして部分結果で済ませる
		searchResult: []boardapi.ProjectEntity{},
	}
	cr := &stubClientRepo{}
	svc := newEstimateTestService(er, pr, cr)

	results, err := svc.FindEstimate(testCtx, FindEstimateQuery{ID: 100, ProjectID: 7})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if pr.getWithGroupCount != 0 {
		t.Errorf("expected GetByIDWithGroup not called (ID priority), got %d", pr.getWithGroupCount)
	}
}
