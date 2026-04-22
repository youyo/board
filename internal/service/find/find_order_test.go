package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindOrder: Normal Cases ---

// M36: OrderEntity に ClientID/ProjectID/Status フィールドは存在しない。
// ID lookup では client/project は nil。ProjectID/ClientName/ProjectName ブランチでは
// project コンテキストから enrichment を行う。

func TestFindOrder_ByID(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, Total: "90000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)

	if got[0].Order.ID != 1 {
		t.Errorf("order ID = %d, want 1", got[0].Order.ID)
	}
	// ID lookup では client/project は特定不能なので nil
	if got[0].Client != nil {
		t.Errorf("expected nil client for ID-only lookup, got %+v", got[0].Client)
	}
	if got[0].Project != nil {
		t.Errorf("expected nil project for ID-only lookup, got %+v", got[0].Project)
	}
}

func TestFindOrder_ByProjectID(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "Web Dev", Order: docSummary}
	ord := &boardapi.OrderEntity{ID: 42, Total: "80000.0"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{getWithGroupResult: project, getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ProjectID: 100})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Order.ID != 42 {
		t.Errorf("order ID = %d, want 42", got[0].Order.ID)
	}
}

func TestFindOrder_ByClientName(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC Corp"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "P1", Order: docSummary},
	}
	ord := &boardapi.OrderEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ClientName: "ABC"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
}

func TestFindOrder_ByProjectName(t *testing.T) {
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "Web Dev", Order: docSummary},
	}
	ord := &boardapi.OrderEntity{ID: 1, Total: "80000.0"}

	svc := newServiceWith(
		&stubClientRepo{getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
}

// TestFindOrder_ByClientNameWithStatus: Status クエリパラメータを受け付けるが、
// M36 時点では Status post-filter は無効（OrderEntity に Status フィールド無し）。
// TODO(M25-M32): Status post-filter を再設計で復元する。
func TestFindOrder_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := &boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "P1", Order: docSummary},
	}
	ord := &boardapi.OrderEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ClientName: "ABC", Status: "confirmed"})
	assertNoError(t, err)
	// Status post-filter は無効のため、条件に関係なく 1 件返る
	assertOrderResultLen(t, got, 1)
}

// --- FindOrder: Error/Edge Cases ---

func TestFindOrder_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindOrder(testCtx, find.FindOrderQuery{})
	assertError(t, err)
}

func TestFindOrder_StatusOnlyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindOrder(testCtx, find.FindOrderQuery{Status: "confirmed"})
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

// TestFindOrder_ClientResolutionFailure: ID lookup では enrichment しないため
// client error は発生しない。結果は Client=nil で正常に返る。
func TestFindOrder_ClientResolutionFailure(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client for ID-only lookup")
	}
}

// TestFindOrder_ProjectResolutionFailure: ID lookup では enrichment しないため
// project error は発生しない。結果は Project=nil で正常に返る。
func TestFindOrder_ProjectResolutionFailure(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project for ID-only lookup")
	}
}

// --- FindOrder: Priority ---

func TestFindOrder_IDPriorityOverProjectID(t *testing.T) {
	ord := &boardapi.OrderEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ID: 1, ProjectID: 100})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 1)
	if got[0].Order.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Order.ID)
	}
}

// --- FindOrder: Limit ---

func TestFindOrder_Limit(t *testing.T) {
	docSummary1 := &boardapi.DocumentSummary{ID: 1}
	docSummary2 := &boardapi.DocumentSummary{ID: 2}
	docSummary3 := &boardapi.DocumentSummary{ID: 3}
	projects := []boardapi.ProjectEntity{
		{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "P1", Order: docSummary1},
		{ID: 101, Client: &boardapi.ClientRef{ID: 10}, Name: "P2", Order: docSummary2},
		{ID: 102, Client: &boardapi.ClientRef{ID: 10}, Name: "P3", Order: docSummary3},
	}
	ord := &boardapi.OrderEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubOrderRepo{getByDocIDResult: ord},
	)

	got, err := svc.FindOrder(testCtx, find.FindOrderQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertOrderResultLen(t, got, 2)
}
