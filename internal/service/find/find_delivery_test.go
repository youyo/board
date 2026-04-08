package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindDelivery: Normal Cases ---

func TestFindDelivery_ByID(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, ClientID: 10, ProjectID: 100, Title: "Delivery A"}
	client := &boardapi.ClientEntity{ID: 10, Name: "Client X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		nil, nil,
		&stubProjectRepo{getResult: project},
		&stubDeliveryRepo{getResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)

	if got[0].Delivery.ID != 1 {
		t.Errorf("delivery ID = %d, want 1", got[0].Delivery.ID)
	}
	if got[0].Client == nil || got[0].Client.ID != 10 {
		t.Error("client not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Error("project not resolved correctly")
	}
}

func TestFindDelivery_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	deliveries := []boardapi.DeliveryEntity{
		{ID: 1, ClientID: 10, Title: "D1"},
		{ID: 2, ClientID: 10, Title: "D2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{searchResult: deliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 2)
}

func TestFindDelivery_ByProjectName(t *testing.T) {
	projects := []boardapi.ProjectEntity{{ID: 100, ClientID: 10, Name: "Web Dev"}}
	deliveries := []boardapi.DeliveryEntity{{ID: 1, ClientID: 10, ProjectID: 100, Title: "D1"}}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubDeliveryRepo{searchResult: deliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
}

func TestFindDelivery_ByText(t *testing.T) {
	allDeliveries := []boardapi.DeliveryEntity{
		{ID: 1, Title: "Final Delivery", Memo: "complete"},
		{ID: 2, Title: "Partial", Memo: "normal"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{listResult: allDeliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{Text: "Final"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
}

func TestFindDelivery_ByStatus(t *testing.T) {
	allDeliveries := []boardapi.DeliveryEntity{
		{ID: 1, Status: "delivered"},
		{ID: 2, Status: "draft"},
		{ID: 3, Status: "delivered"},
	}

	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{listResult: allDeliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{Status: "delivered"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 2)
}

func TestFindDelivery_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	deliveries := []boardapi.DeliveryEntity{
		{ID: 1, ClientID: 10, Status: "delivered", Title: "D1"},
		{ID: 2, ClientID: 10, Status: "draft", Title: "D2"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{searchResult: deliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ClientName: "ABC", Status: "delivered"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
}

// --- FindDelivery: Error/Edge Cases ---

func TestFindDelivery_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{})
	assertError(t, err)
}

func TestFindDelivery_NotFoundByID(t *testing.T) {
	svc := newServiceWith(nil, nil, nil, nil,
		&stubDeliveryRepo{err: errors.New("not found")},
	)
	_, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 999})
	assertError(t, err)
}

func TestFindDelivery_NoMatchByClientName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil}, nil, nil, nil,
		&stubDeliveryRepo{},
	)
	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ClientName: "nonexistent"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 0)
}

func TestFindDelivery_ClientResolutionFailure(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{err: errors.New("client error")}, nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubDeliveryRepo{getResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client on resolution failure")
	}
}

func TestFindDelivery_ProjectResolutionFailure(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, ClientID: 10, ProjectID: 100}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}}, nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubDeliveryRepo{getResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindDelivery: Priority ---

func TestFindDelivery_IDPriorityOverClientName(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, ClientID: 10, Title: "By ID"}
	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}, searchResult: []boardapi.ClientEntity{{ID: 20}}},
		nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{getResult: del, searchResult: []boardapi.DeliveryEntity{{ID: 99}}},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1, ClientName: "ABC"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Delivery.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Delivery.ID)
	}
}

// --- FindDelivery: Limit ---

func TestFindDelivery_Limit(t *testing.T) {
	deliveries := []boardapi.DeliveryEntity{
		{ID: 1, Title: "D1"}, {ID: 2, Title: "D2"}, {ID: 3, Title: "D3"},
	}
	svc := newServiceWith(
		&stubClientRepo{}, nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{listResult: deliveries},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{Text: "D", Limit: 2})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 2)
}
