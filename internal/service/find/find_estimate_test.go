package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindEstimate: Normal Cases ---

func TestFindEstimate_ByID(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "Estimate A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)

	if got[0].Estimate.ID != 1 {
		t.Errorf("estimate ID = %d, want 1", got[0].Estimate.ID)
	}
	if got[0].Client == nil || got[0].Client.ID != 10 {
		t.Errorf("client not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Errorf("project not resolved correctly")
	}
}

func TestFindEstimate_ByProjectID(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Estimate: docSummary}
	est := &boardapi.EstimateEntity{ID: 42, ClientID: 10, ProjectID: 100, Title: "E1"}

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
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "E1"}

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
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "E1"}

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

func TestFindEstimate_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Estimate: docSummary},
	}
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, Status: "approved", Title: "E1"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10, Name: "ABC"}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ClientName: "ABC", Status: "approved"})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Estimate.Status != "approved" {
		t.Errorf("status = %q, want approved", got[0].Estimate.Status)
	}
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

func TestFindEstimate_ClientResolutionFailure(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, ProjectID: 100}

	svc := newServiceWith(
		&stubClientRepo{err: errors.New("client error")},
		nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client on resolution failure")
	}
}

func TestFindEstimate_ProjectResolutionFailure(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, ProjectID: 100}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ID: 1})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindEstimate: Priority Cases ---

func TestFindEstimate_IDPriorityOverProjectID(t *testing.T) {
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, Title: "By ID"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
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
	est := &boardapi.EstimateEntity{ID: 1, ClientID: 10, Title: "E"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubEstimateRepo{getByDocIDResult: est},
	)

	got, err := svc.FindEstimate(testCtx, find.FindEstimateQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertEstimateResultLen(t, got, 2)
}
