package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindOrder: Normal Cases ---

func TestFindOrder_ByID(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "Order A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
		&stubOrderRepo{getResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)

	if got[0].Order.ID != 1 {
		t.Errorf("order ID = %d, want 1", got[0].Order.ID)
	}
	if got[0].Client == nil || got[0].Client.ID != 10 {
		t.Error("client not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Error("project not resolved correctly")
	}
}

func TestFindOrder_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	orders := []boardapi.OrderEntity{
		{ID: 1, ClientID: 10, Title: "O1"},
		{ID: 2, ClientID: 10, Title: "O2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{searchResult: orders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 2)
}

func TestFindOrder_ByProjectName(t *testing.T) {
	projects := []boardapi.ProjectEntity{{ID: 100, ClientID: 10, Name: "Web Dev"}}
	orders := []boardapi.OrderEntity{{ID: 1, ClientID: 10, ProjectID: 100, Title: "O1"}}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubOrderRepo{searchResult: orders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
}

func TestFindOrder_ByText(t *testing.T) {
	allOrders := []boardapi.OrderEntity{
		{ID: 1, Title: "Server Purchase", Memo: "important"},
		{ID: 2, Title: "License", Memo: "normal"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{listResult: allOrders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{Text: "Server"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
}

func TestFindOrder_ByStatus(t *testing.T) {
	allOrders := []boardapi.OrderEntity{
		{ID: 1, Status: "confirmed"},
		{ID: 2, Status: "draft"},
		{ID: 3, Status: "confirmed"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{listResult: allOrders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{Status: "confirmed"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 2)
}

func TestFindOrder_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	orders := []boardapi.OrderEntity{
		{ID: 1, ClientID: 10, Status: "confirmed", Title: "O1"},
		{ID: 2, ClientID: 10, Status: "draft", Title: "O2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{searchResult: orders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ClientName: "ABC", Status: "confirmed"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
}

// --- FindOrder: Error/Edge Cases ---

func TestFindOrder_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindOrder(testCtx, find.FindOrderQuery{})
	assertError(t, err)
}

func TestFindOrder_NotFoundByID(t *testing.T) {
	svc := newServiceWith(nil, nil, nil, nil,
		&stubOrderRepo{err: errors.New("not found")},
	)
	_, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 999})
	assertError(t, err)
}

func TestFindOrder_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil}, nil, nil, nil,
		&stubOrderRepo{},
	)
	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 0)
}

func TestFindOrder_ClientResolutionFailure(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{err: errors.New("client error")}, nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubOrderRepo{getResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client on resolution failure")
	}
}

func TestFindOrder_ProjectResolutionFailure(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}}, nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubOrderRepo{getResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindOrder: Priority ---

func TestFindOrder_IDPriorityOverClientName(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, ClientID: 10, Title: "By ID"}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}, searchResult: []boardapi.ClientEntity{{ID: 20}}},
		nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{getResult: ord, searchResult: []boardapi.OrderEntity{{ID: 99}}},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1, ClientName: "ABC"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Order.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Order.ID)
	}
}

// --- FindOrder: Limit ---

func TestFindOrder_Limit(t *testing.T) {
	orders := []boardapi.OrderEntity{
		{ID: 1, Title: "O1"}, {ID: 2, Title: "O2"}, {ID: 3, Title: "O3"},
	}
	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{listResult: orders},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{Text: "O", Limit: 2})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 2)
}
