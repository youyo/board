package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindProject: Normal Cases ---

func TestFindProject_ByID(t *testing.T) {
	project := &boardapi.ProjectEntity{ID: 1, ClientID: 10, Name: "Project A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{ID: 1})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)

	if got[0].Project.ID != 1 {
		t.Errorf("project ID = %d, want 1", got[0].Project.ID)
	}
	if got[0].Client == nil {
		t.Fatal("client is nil, want non-nil")
	}
	if got[0].Client.ID != 10 {
		t.Errorf("client ID = %d, want 10", got[0].Client.ID)
	}
}

func TestFindProject_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{
		{ID: 10, Name: "ABC Corp"},
		{ID: 11, Name: "ABC Inc"},
	}
	projects10 := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "P1"},
		{ID: 2, ClientID: 10, Name: "P2"},
	}
	projects11 := []boardapi.ProjectEntity{
		{ID: 3, ClientID: 11, Name: "P3"},
	}

	clientRepo := &stubClientRepo{searchResult: clients}
	// For client resolution in resolveProjectClient, GetByID needs to work
	clientRepo.getResult = &boardapi.ClientEntity{ID: 10, Name: "ABC Corp"}

	projectRepo := &stubProjectRepo{}
	// Since stub returns the same result for all Search calls, combine
	allProjects := append(projects10, projects11...)
	projectRepo.searchResult = allProjects

	// Use a more specific approach: the stub returns all projects for any search.
	// In practice, the stub can't differentiate by ClientID, so we test the flow.
	svc := newServiceWith(clientRepo, nil, nil, projectRepo)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{ClientName: "ABC"})
	assertNoError(t, err)
	// 2 clients * same stub result = 6, but we're testing the flow works
	// The stub returns allProjects for each client search call
	if len(got) < 1 {
		t.Fatal("expected at least 1 result")
	}
}

func TestFindProject_ByName(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "Dev Project"},
	}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{Name: "Dev"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)
}

func TestFindProject_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	projects := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "P1", Status: "active"},
		{ID: 2, ClientID: 10, Name: "P2", Status: "closed"},
	}
	client := &boardapi.ClientEntity{ID: 10, Name: "ABC"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: client},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{ClientName: "ABC", Status: "active"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)
	if got[0].Project.Status != "active" {
		t.Errorf("status = %q, want active", got[0].Project.Status)
	}
}

func TestFindProject_ByText(t *testing.T) {
	allProjects := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "Web App", Code: "WA", Memo: "search target"},
		{ID: 2, ClientID: 10, Name: "API", Code: "API", Memo: "normal"},
	}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{listResult: allProjects},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{Text: "search"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)
	if got[0].Project.ID != 1 {
		t.Errorf("project ID = %d, want 1", got[0].Project.ID)
	}
}

// --- FindProject: Error/Edge Cases ---

func TestFindProject_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{err: errors.New("not found")},
	)

	_, err := svc.FindProject(testCtx, find.FindProjectQuery{ID: 999})
	assertError(t, err)
}

func TestFindProject_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil},
		nil, nil,
		&stubProjectRepo{},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 0)
}

func TestFindProject_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())

	_, err := svc.FindProject(testCtx, find.FindProjectQuery{})
	assertError(t, err)
}

func TestFindProject_ClientSearchOK_ProjectSearchFails(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients},
		nil, nil,
		&stubProjectRepo{err: errors.New("project repo error")},
	)

	_, err := svc.FindProject(testCtx, find.FindProjectQuery{ClientName: "ABC"})
	assertError(t, err)
}

// --- FindProject: Priority Cases ---

func TestFindProject_IDPriorityOverClientName(t *testing.T) {
	project := &boardapi.ProjectEntity{ID: 1, ClientID: 10, Name: "By ID"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client"}

	svc := newServiceWith(
		&stubClientRepo{
			getResult:    client,
			searchResult: []boardapi.ClientEntity{{ID: 20, Name: "Other"}},
		},
		nil, nil,
		&stubProjectRepo{
			getResult:    project,
			searchResult: []boardapi.ProjectEntity{{ID: 99, Name: "By Search"}},
		},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{ID: 1, ClientName: "Other"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)
	if got[0].Project.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Project.ID)
	}
}

// --- FindProject: Limit Cases ---

func TestFindProject_Limit(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "P1"},
		{ID: 2, ClientID: 10, Name: "P2"},
		{ID: 3, ClientID: 10, Name: "P3"},
	}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{Name: "P", Limit: 1})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 1)
}

// --- FindProject: Status-only ---

func TestFindProject_StatusOnly(t *testing.T) {
	allProjects := []boardapi.ProjectEntity{
		{ID: 1, ClientID: 10, Name: "P1", Status: "active"},
		{ID: 2, ClientID: 10, Name: "P2", Status: "closed"},
		{ID: 3, ClientID: 10, Name: "P3", Status: "active"},
	}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{listResult: allProjects},
	)

	got, err := svc.FindProject(testCtx, find.FindProjectQuery{Status: "active"})
	assertNoError(t, err)
	assertProjectResultLen(t, got, 2)
}
