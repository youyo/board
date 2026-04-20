package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindEstimate: Normal Cases ---

// M35: EstimateEntity に ClientID/ProjectID/Status フィールドは存在しない。
// ID lookup では client/project は nil。ProjectID/ClientName/ProjectName ブランチでは
// project コンテキストから enrichment を行う。

func TestFindEstimate_ByID(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, Total: "90000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)

	if got[0].Estimate.ID != 1 {
		t.Errorf("estimate ID = %d, want 1", got[0].Estimate.ID)
	}
	// ID lookup では client/project は特定不能なので nil
	if got[0].Client != nil {
		t.Errorf("expected nil client for ID-only lookup, got %+v", got[0].Client)
	}
	if got[0].Project != nil {
		t.Errorf("expected nil project for ID-only lookup, got %+v", got[0].Project)
	}
}

func TestFindEstimate_ByProjectID(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Estimate: docSummary}
	est := &boardapi.EstimateEntity{ID: 42, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{getWithGroupResult: project, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ProjectID: 100})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Estimate.ID != 42 {
		t.Errorf("estimate ID = %d, want 42", got[0].Estimate.ID)
	}
}

func TestFindEstimate_ByProjectID_NoEstimate(t *testing.T) {
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Estimate: nil}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{getWithGroupResult: project},
		&stubEstimateRepo{},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ProjectID: 100})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 0)
}

func TestFindEstimate_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Estimate: docSummary},
	}
	est := &boardapi.EstimateEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10, Name: "ABC Corp"}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100, Name: "P1"}},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
}

func TestFindEstimate_ByProjectName(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "Web Dev", Estimate: docSummary},
	}
	est := &boardapi.EstimateEntity{ID: 1, Total: "80000.0"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10, Name: "Client"}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100, Name: "Web Dev"}},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
}

// TestFindEstimate_ByClientNameWithStatus は Status クエリパラメータを受け付けるが、
// M35 時点では Status post-filter は無効（EstimateEntity に Status フィールド無し）。
// Status フィールドは FindEstimateQuery に残るが post-filter は TODO(M25-M32)。
// ここでは Status を指定しても結果が返ることを確認する。
func TestFindEstimate_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Estimate: docSummary},
	}
	est := &boardapi.EstimateEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10, Name: "ABC"}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ClientName: "ABC", Status: "approved"})
	assertNoError(t, err)
	// Status post-filter は無効のため、条件に関係なく 1 件返る
	assertEstimateResultLen(t, got, 1)
}

// --- FindEstimate: Error/Edge Cases ---

func TestFindEstimate_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{})
	assertError(t, err)
}

func TestFindEstimate_StatusOnlyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{Status: "approved"})
	assertError(t, err)
}

func TestFindEstimate_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubEstimateRepo{err: errors.New("not found")},
	)

	_, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 999})
	assertError(t, err)
}

func TestFindEstimate_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil},
		nil, nil, nil,
		&stubEstimateRepo{},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 0)
}

// TestFindEstimate_ClientResolutionFailure: ID lookup では enrichment しないため
// client error は発生しない。結果は Client=nil, Project=nil で正常に返る。
func TestFindEstimate_ClientResolutionFailure(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, Total: "90000.0"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client for ID-only lookup")
	}
}

// TestFindEstimate_ProjectResolutionFailure: ID lookup では enrichment しないため
// project error は発生しない。結果は Project=nil で正常に返る。
func TestFindEstimate_ProjectResolutionFailure(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, Total: "90000.0"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project for ID-only lookup")
	}
}

// --- FindEstimate: Priority Cases ---

func TestFindEstimate_IDPriorityOverProjectID(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, Total: "90000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1, ProjectID: 100})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Estimate.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Estimate.ID)
	}
}

// --- FindEstimate: Limit Cases ---

func TestFindEstimate_Limit(t *testing.T) {
	docSummary1 := &boardapi.DocumentSummary{ID: 1}
	docSummary2 := &boardapi.DocumentSummary{ID: 2}
	docSummary3 := &boardapi.DocumentSummary{ID: 3}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Estimate: docSummary1},
		{ID: 101, ClientID: 10, Name: "P2", Estimate: docSummary2},
		{ID: 102, ClientID: 10, Name: "P3", Estimate: docSummary3},
	}
	est := &boardapi.EstimateEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 2)
}
