package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindInvoice: Normal Cases ---

func TestFindInvoice_ByID(t *testing.T) {
	inv := &boardapi.InvoiceEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "Invoice A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
		&stubInvoiceRepo{getResult: inv},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ID: 1})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)

	if got[0].Invoice.ID != 1 {
		t.Errorf("invoice ID = %d, want 1", got[0].Invoice.ID)
	}
	if got[0].Client == nil || got[0].Client.ID != 10 {
		t.Error("client not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Error("project not resolved correctly")
	}
}

func TestFindInvoice_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	invoices := []boardapi.InvoiceEntity{
		{ID: 1, ClientID: 10, Title: "I1"},
		{ID: 2, ClientID: 10, Title: "I2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{searchResult: invoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 2)
}

func TestFindInvoice_ByProjectName(t *testing.T) {
	// M44: ClientID 廃止、Client nested に統合
	projects := []boardapi.ProjectEntity{{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "Web Dev"}}
	invoices := []boardapi.InvoiceEntity{{ID: 1, ClientID: 10, ProjectID: 100, Title: "I1"}}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubInvoiceRepo{searchResult: invoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
}

func TestFindInvoice_ByText(t *testing.T) {
	allInvoices := []boardapi.InvoiceEntity{
		{ID: 1, ClientID: 10, Title: "Monthly Invoice", Memo: "important"},
		{ID: 2, ClientID: 10, Title: "Quarterly", Memo: "normal"},
	}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{listResult: allInvoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{Text: "Monthly"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
}

func TestFindInvoice_ByStatus(t *testing.T) {
	allInvoices := []boardapi.InvoiceEntity{
		{ID: 1, Status: "sent"},
		{ID: 2, Status: "draft"},
		{ID: 3, Status: "sent"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{listResult: allInvoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{Status: "sent"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 2)
}

func TestFindInvoice_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	invoices := []boardapi.InvoiceEntity{
		{ID: 1, ClientID: 10, Status: "sent", Title: "I1"},
		{ID: 2, ClientID: 10, Status: "draft", Title: "I2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{searchResult: invoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ClientName: "ABC", Status: "sent"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
}

// --- FindInvoice: Error/Edge Cases ---

func TestFindInvoice_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{})
	assertError(t, err)
}

func TestFindInvoice_NotFoundByID(t *testing.T) {
	svc := newServiceWith(nil, nil, nil, nil,
		&stubInvoiceRepo{err: errors.New("not found")},
	)
	_, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ID: 999})
	assertError(t, err)
}

func TestFindInvoice_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil}, nil, nil, nil,
		&stubInvoiceRepo{},
	)
	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 0)
}

func TestFindInvoice_ClientResolutionFailure(t *testing.T) {
	inv := &boardapi.InvoiceEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{err: errors.New("client error")}, nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubInvoiceRepo{getResult: inv},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ID: 1})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client on resolution failure")
	}
}

func TestFindInvoice_ProjectResolutionFailure(t *testing.T) {
	inv := &boardapi.InvoiceEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}}, nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubInvoiceRepo{getResult: inv},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ID: 1})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindInvoice: Priority ---

func TestFindInvoice_IDPriorityOverClientName(t *testing.T) {
	inv := &boardapi.InvoiceEntity{ID: 1, ClientID: 10, Title: "By ID"}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}, searchResult: []boardapi.ClientEntity{{ID: 20}}},
		nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{getResult: inv, searchResult: []boardapi.InvoiceEntity{{ID: 99}}},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{ID: 1, ClientName: "ABC"})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 1)
	if got[0].Invoice.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Invoice.ID)
	}
}

// --- FindInvoice: Limit ---

func TestFindInvoice_Limit(t *testing.T) {
	invoices := []boardapi.InvoiceEntity{
		{ID: 1, Title: "I1"}, {ID: 2, Title: "I2"}, {ID: 3, Title: "I3"},
	}
	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubInvoiceRepo{listResult: invoices},
	)

	got, err := svc.FindInvoice(testCtx, find.FindInvoiceQuery{Text: "I", Limit: 2})
	assertNoError(t, err)
	assertInvoiceResultLen(t, got, 2)
}
