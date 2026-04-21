package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindReceipt: Normal Cases ---

// M38: ReceiptEntity に ClientID/ProjectID/Status フィールドは存在しない。
// ReceiptDate は実在フィールドなので引き続き利用可能。
// ID lookup では client/project は nil。ProjectID/ClientName/ProjectName ブランチでは
// project コンテキストから enrichment を行う。
//
// M29 FIX: BOARD API は response_group=receipt で "receipts" 複数形配列を返す。
// モックデータも ProjectEntity.Receipts ([]DocumentSummary) で設定すること。

func TestFindReceipt_ByID(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "90000.0", ReceiptDate: "2026-06-30"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)

	if got[0].Receipt.ID != 1 {
		t.Errorf("receipt ID = %d, want 1", got[0].Receipt.ID)
	}
	// ID lookup では client/project は特定不能なので nil
	if got[0].Client != nil {
		t.Errorf("expected nil client for ID-only lookup, got %+v", got[0].Client)
	}
	if got[0].Project != nil {
		t.Errorf("expected nil project for ID-only lookup, got %+v", got[0].Project)
	}
}

func TestFindReceipt_ByProjectID(t *testing.T) {
	docSummary := boardapi.DocumentSummary{ID: 42}
	project := &boardapi.ProjectEntity{ID: 100, ClientID: 10, Name: "Web Dev", Receipts: []boardapi.DocumentSummary{docSummary}}
	rec := &boardapi.ReceiptEntity{ID: 42, Total: "80000.0", ReceiptDate: "2026-06-30"}

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
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipts: []boardapi.DocumentSummary{docSummary}},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "50000.0"}

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
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "Web Dev", Receipts: []boardapi.DocumentSummary{docSummary}},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "80000.0"}

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

// TestFindReceipt_ByClientNameWithStatus: Status クエリパラメータを受け付けるが、
// M38 時点では Status post-filter は無効（ReceiptEntity に Status フィールド無し）。
// TODO(M25-M32): Status post-filter を再設計で復元する。
func TestFindReceipt_ByClientNameWithStatus(t *testing.T) {
	clients := []boardapi.ClientEntity{{ID: 10, Name: "ABC"}}
	docSummary := boardapi.DocumentSummary{ID: 1}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipts: []boardapi.DocumentSummary{docSummary}},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients, getResult: &boardapi.ClientEntity{ID: 10}},
		nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ClientName: "ABC", Status: "confirmed"})
	assertNoError(t, err)
	// Status post-filter は無効のため、条件に関係なく 1 件返る
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

// TestFindReceipt_ClientResolutionFailure: ID lookup では enrichment しないため
// client error は発生しない。結果は Client=nil で正常に返る。
func TestFindReceipt_ClientResolutionFailure(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Client != nil {
		t.Error("expected nil client for ID-only lookup")
	}
}

// TestFindReceipt_ProjectResolutionFailure: ID lookup では enrichment しないため
// project error は発生しない。結果は Project=nil で正常に返る。
func TestFindReceipt_ProjectResolutionFailure(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ID: 1})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project for ID-only lookup")
	}
}

// --- FindReceipt: Priority ---

func TestFindReceipt_IDPriorityOverProjectID(t *testing.T) {
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "90000.0"}
	svc := newServiceWith(
		nil, nil, nil,
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
	docSummary1 := boardapi.DocumentSummary{ID: 1}
	docSummary2 := boardapi.DocumentSummary{ID: 2}
	docSummary3 := boardapi.DocumentSummary{ID: 3}
	projects := []boardapi.ProjectEntity{
		{ID: 100, ClientID: 10, Name: "P1", Receipts: []boardapi.DocumentSummary{docSummary1}},
		{ID: 101, ClientID: 10, Name: "P2", Receipts: []boardapi.DocumentSummary{docSummary2}},
		{ID: 102, ClientID: 10, Name: "P3", Receipts: []boardapi.DocumentSummary{docSummary3}},
	}
	rec := &boardapi.ReceiptEntity{ID: 1, Total: "50000.0"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects},
		&stubReceiptRepo{getByDocIDResult: rec},
	)

	got, err := svc.FindReceipt(testCtx, find.FindReceiptQuery{ProjectName: "P", Limit: 2})
	assertNoError(t, err)
	assertReceiptResultLen(t, got, 2)
}
