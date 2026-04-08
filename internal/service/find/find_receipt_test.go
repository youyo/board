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
		&stubReceiptRepo{getResult: rec},
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

func TestFindReceipt_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	receipts := []boardapi.ReceiptEntity{
		{ID: 1, ClientID: 10, Title: "R1"},
		{ID: 2, ClientID: 10, Title: "R2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{searchResult: receipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 2)
}

func TestFindReceipt_ByProjectName(t *testing.T) {
	projects := []boardapi.ProjectEntity{{ID: 100, ClientID: 10, Name: "Web Dev"}}
	receipts := []boardapi.ReceiptEntity{{ID: 1, ClientID: 10, ProjectID: 100, Title: "R1"}}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubReceiptRepo{searchResult: receipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

func TestFindReceipt_ByText(t *testing.T) {
	allReceipts := []boardapi.ReceiptEntity{
		{ID: 1, Title: "Payment Receipt", Memo: "complete"},
		{ID: 2, Title: "Deposit", Memo: "normal"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{listResult: allReceipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{Text: "Payment"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

func TestFindReceipt_ByStatus(t *testing.T) {
	allReceipts := []boardapi.ReceiptEntity{
		{ID: 1, Status: "issued"},
		{ID: 2, Status: "draft"},
		{ID: 3, Status: "issued"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{listResult: allReceipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{Status: "issued"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 2)
}

func TestFindReceipt_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	receipts := []boardapi.ReceiptEntity{
		{ID: 1, ClientID: 10, Status: "issued", Title: "R1"},
		{ID: 2, ClientID: 10, Status: "draft", Title: "R2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{searchResult: receipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "ABC", Status: "issued"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
}

// --- FindReceipt: Error/Edge Cases ---

func TestFindReceipt_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{})
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
		&stubReceiptRepo{getResult: rec},
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
		&stubReceiptRepo{getResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindReceipt: Priority ---

func TestFindReceipt_IDPriorityOverClientName(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, ClientID: 10, Title: "By ID"}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}, searchResult: []boardapi.ClientEntity{{ID: 20}}},
		nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{getResult: rec, searchResult: []boardapi.ReceiptEntity{{ID: 99}}},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1, ClientName: "ABC"})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Receipt.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Receipt.ID)
	}
}

// --- FindReceipt: Limit ---

func TestFindReceipt_Limit(t *testing.T) {
	receipts := []boardapi.ReceiptEntity{
		{ID: 1, Title: "R1"}, {ID: 2, Title: "R2"}, {ID: 3, Title: "R3"},
	}
	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{listResult: receipts},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{Text: "R", Limit: 2})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 2)
}
