package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindReceipt: Normal Cases ---

func TestFindReceipt_ByID(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "Receipt A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)

	if got[0].Receipt.ID != 1 {
		t.Errorf("receipt ID = %d, want 1", got[0].Receipt.ID)
	}
	if got[0].Client == nil || got[0].Client.ID != 10 {
		t.Error("client not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Error("project not resolved correctly")
	}
}

func TestFindReceipt_ByProjectID(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Receipt: docSummary}
	rec := &boardapi.ReceiptEntity{ID: 42, ClientID: 10, ProjectID: 100, Title: "R1"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{getWithGroupResult: project, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ProjectID: 100})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Receipt.ID != 42 {
		t.Errorf("receipt ID = %d, want 42", got[0].Receipt.ID)
	}
}

func TestFindReceipt_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipt: docSummary},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "R1"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

func TestFindReceipt_ByProjectName(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "Web Dev", Receipt: docSummary},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "R1"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

func TestFindReceipt_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipt: docSummary},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, Status: "confirmed", Title: "R1"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "ABC", Status: "confirmed"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

// --- FindReceipt: Error/Edge Cases ---

func TestFindReceipt_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{})
	assertError(t, err)
}

func TestFindReceipt_StatusOnlyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{Status: "confirmed"})
	assertError(t, err)
}

func TestFindReceipt_NotFoundByID(t *testing.T) {
	svc := newServiceWith(nil, nil, nil, nil,
		&stubReceiptRepo{err: errors.New("not found")},
	)
	_, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 999})
	assertError(t, err)
}

func TestFindReceipt_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil}, nil, nil, nil,
		&stubReceiptRepo{},
	)
	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 0)
}

func TestFindReceipt_ClientResolutionFailure(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{err: errors.New("client error")}, nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client on resolution failure")
	}
}

func TestFindReceipt_ProjectResolutionFailure(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}}, nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindReceipt: Priority ---

func TestFindReceipt_IDPriorityOverProjectID(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, Title: "By ID"}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1, ProjectID: 100})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Receipt.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Receipt.ID)
	}
}

// --- FindReceipt: Limit ---

func TestFindReceipt_Limit(t *testing.T) {
	docSummary1 := &boardapi.DocumentSummary{ID: 1}
	docSummary2 := &boardapi.DocumentSummary{ID: 2}
	docSummary3 := &boardapi.DocumentSummary{ID: 3}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipt: docSummary1},
		{ID: 101, ClientID: 10, Name: "P2", Receipt: docSummary2},
		{ID: 102, ClientID: 10, Name: "P3", Receipt: docSummary3},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, Title: "R"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 2)
}
