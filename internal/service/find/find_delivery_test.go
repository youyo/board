package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindDelivery: Normal Cases ---

// M37: DeliveryEntity に ClientID/ProjectID/Status フィールドは存在しない。
// DeliveryDate は実在フィールドなので引き続き利用可能。
// ID lookup では client/project は nil。ProjectID/ClientName/ProjectName ブランチでは
// project コンテキストから enrichment を行う。
//
// M28 FIX: BOARD API は response_group=delivery で "deliveries" 複数形配列を返す。
// モックデータも ProjectEntity.Deliveries ([]DocumentSummary) で設定すること。

func TestFindDelivery_ByID(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, Total: "90000.0", DeliveryDate: "2026-06-30"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)

	if got[0].Delivery.ID != 1 {
		t.Errorf("delivery ID = %d, want 1", got[0].Delivery.ID)
	}
	// ID lookup では client/project は特定不能なので nil
	if got[0].Client != nil {
		t.Errorf("expected nil client for ID-only lookup, got %+v", got[0].Client)
	}
	if got[0].Project != nil {
		t.Errorf("expected nil project for ID-only lookup, got %+v", got[0].Project)
	}
}

func TestFindDelivery_ByProjectID(t *testing.T) {
	docSummary := boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Deliveries: []boardapi.DocumentSummary{docSummary}}
	del := &boardapi.DeliveryEntity{ID: 42, Total: "80000.0", DeliveryDate: "2026-06-30"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{getWithGroupResult: project, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ProjectID: 100})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Delivery.ID != 42 {
		t.Errorf("delivery ID = %d, want 42", got[0].Delivery.ID)
	}
}

func TestFindDelivery_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Deliveries: []boardapi.DocumentSummary{docSummary}},
	}
	del := &boardapi.DeliveryEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
}

func TestFindDelivery_ByProjectName(t *testing.T) {
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "Web Dev", Deliveries: []boardapi.DocumentSummary{docSummary}},
	}
	del := &boardapi.DeliveryEntity{ID: 1, Total: "80000.0"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
}

// TestFindDelivery_ByClientNameWithStatus: Status クエリパラメータを受け付けるが、
// M37 時点では Status post-filter は無効（DeliveryEntity に Status フィールド無し）。
// TODO(M25-M32): Status post-filter を再設計で復元する。
func TestFindDelivery_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Deliveries: []boardapi.DocumentSummary{docSummary}},
	}
	del := &boardapi.DeliveryEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ClientName: "ABC", Status: "delivered"})
	assertNoError(t, err)
	// Status post-filter は無効のため、条件に関係なく 1 件返る
	assertDeliveryResultLen(t, got, 1)
}

// --- FindDelivery: Error/Edge Cases ---

func TestFindDelivery_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{})
	assertError(t, err)
}

func TestFindDelivery_StatusOnlyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{Status: "delivered"})
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

// TestFindDelivery_ClientResolutionFailure: ID lookup では enrichment しないため
// client error は発生しない。結果は Client=nil で正常に返る。
func TestFindDelivery_ClientResolutionFailure(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client for ID-only lookup")
	}
}

// TestFindDelivery_ProjectResolutionFailure: ID lookup では enrichment しないため
// project error は発生しない。結果は Project=nil で正常に返る。
func TestFindDelivery_ProjectResolutionFailure(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project for ID-only lookup")
	}
}

// --- FindDelivery: Priority ---

func TestFindDelivery_IDPriorityOverProjectID(t *testing.T) {
	del := &boardapi.DeliveryEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ID: 1, ProjectID: 100})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 1)
	if got[0].Delivery.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Delivery.ID)
	}
}

// --- FindDelivery: Limit ---

func TestFindDelivery_Limit(t *testing.T) {
	docSummary1 := boardapi.DocumentSummary{ID: 1}
	docSummary2 := boardapi.DocumentSummary{ID: 2}
	docSummary3 := boardapi.DocumentSummary{ID: 3}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Deliveries: []boardapi.DocumentSummary{docSummary1}},
		{ID: 101, ClientID: 10, Name: "P2", Deliveries: []boardapi.DocumentSummary{docSummary2}},
		{ID: 102, ClientID: 10, Name: "P3", Deliveries: []boardapi.DocumentSummary{docSummary3}},
	}
	del := &boardapi.DeliveryEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubDeliveryRepo{getByDocIDResult: del},
	)

	got, err := svc.FindDelivery(testCtx, find.FindDeliveryQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertDeliveryResultLen(t, got, 2)
}
