package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindPurchaseOrder: Normal Cases ---

func TestFindPurchaseOrder_ByID(t *testing.T) {
	po := &boardapi.PurchaseOrderEntity{ID: 1, VendorID: 10, ProjectID: 100, Title: "PO A"}
	vendor := &boardapi.VendorEntity{ID: 10, Name: "Vendor X"}
	project := &boardapi.ProjectEntity{ID: 100, Name: "Project Y"}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{getResult: project},
		&stubVendorRepo{getResult: vendor},
		&stubPurchaseOrderRepo{getResult: po},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ID: 1})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)

	if got[0].PurchaseOrder.ID != 1 {
		t.Errorf("purchase order ID = %d, want 1", got[0].PurchaseOrder.ID)
	}
	if got[0].Vendor == nil || got[0].Vendor.ID != 10 {
		t.Errorf("vendor not resolved correctly")
	}
	if got[0].Project == nil || got[0].Project.ID != 100 {
		t.Errorf("project not resolved correctly")
	}
}

func TestFindPurchaseOrder_ByVendorName(t *testing.T) {
	vendors := []boardapi.VendorEntity{{ID: 10, Name: "ABC Corp"}}
	pos := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, ProjectID: 100, Title: "PO1"},
		{ID: 2, VendorID: 10, ProjectID: 101, Title: "PO2"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors, getResult: &boardapi.VendorEntity{ID: 10, Name: "ABC Corp"}},
		&stubPurchaseOrderRepo{searchResult: pos},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{VendorName: "ABC"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 2)
}

func TestFindPurchaseOrder_ByProjectName(t *testing.T) {
	// M44: ClientID 廃止、Client nested に統合
	projects := []boardapi.ProjectEntity{{ID: 100, Client: &boardapi.ClientRef{ID: 10}, Name: "Web Dev"}}
	pos := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, ProjectID: 100, Title: "PO1"},
	}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{searchResult: projects, getResult: &boardapi.ProjectEntity{ID: 100, Name: "Web Dev"}},
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPurchaseOrderRepo{searchResult: pos},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ProjectName: "Web"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
}

func TestFindPurchaseOrder_ByText(t *testing.T) {
	allPOs := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, Title: "Server Purchase", Memo: "important"},
		{ID: 2, VendorID: 10, Title: "License Renewal", Memo: "normal"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPurchaseOrderRepo{listResult: allPOs},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{Text: "Server"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
	if got[0].PurchaseOrder.ID != 1 {
		t.Errorf("purchase order ID = %d, want 1", got[0].PurchaseOrder.ID)
	}
}

func TestFindPurchaseOrder_ByStatus(t *testing.T) {
	allPOs := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, Status: "approved"},
		{ID: 2, VendorID: 10, Status: "draft"},
		{ID: 3, VendorID: 10, Status: "approved"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPurchaseOrderRepo{listResult: allPOs},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{Status: "approved"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 2)
}

func TestFindPurchaseOrder_ByVendorNameWithStatus(t *testing.T) {
	vendors := []boardapi.VendorEntity{{ID: 10, Name: "ABC"}}
	pos := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, Status: "approved", Title: "PO1"},
		{ID: 2, VendorID: 10, Status: "draft", Title: "PO2"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors, getResult: &boardapi.VendorEntity{ID: 10, Name: "ABC"}},
		&stubPurchaseOrderRepo{searchResult: pos},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{VendorName: "ABC", Status: "approved"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
	if got[0].PurchaseOrder.Status != "approved" {
		t.Errorf("status = %q, want approved", got[0].PurchaseOrder.Status)
	}
}

// --- FindPurchaseOrder: Error/Edge Cases ---

func TestFindPurchaseOrder_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{})
	assertError(t, err)
}

func TestFindPurchaseOrder_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubPurchaseOrderRepo{err: errors.New("not found")},
	)

	_, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ID: 999})
	assertError(t, err)
}

func TestFindPurchaseOrder_NoMatchByVendorName(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: nil},
		&stubPurchaseOrderRepo{},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{VendorName: "nonexistent"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 0)
}

func TestFindPurchaseOrder_VendorResolutionFailure(t *testing.T) {
	po := &boardapi.PurchaseOrderEntity{ID: 1, VendorID: 10, ProjectID: 100}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{getResult: &boardapi.ProjectEntity{ID: 100}},
		&stubVendorRepo{err: errors.New("vendor error")},
		&stubPurchaseOrderRepo{getResult: po},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ID: 1})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
	if got[0].Vendor != nil {
		t.Error("expected nil vendor on resolution failure")
	}
}

func TestFindPurchaseOrder_ProjectResolutionFailure(t *testing.T) {
	po := &boardapi.PurchaseOrderEntity{ID: 1, VendorID: 10, ProjectID: 100}

	svc := newServiceWith(
		nil, nil, nil,
		&stubProjectRepo{err: errors.New("project error")},
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPurchaseOrderRepo{getResult: po},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ID: 1})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
	if got[0].Project != nil {
		t.Error("expected nil project on resolution failure")
	}
}

// --- FindPurchaseOrder: Priority Cases ---

func TestFindPurchaseOrder_IDPriorityOverVendorName(t *testing.T) {
	po := &boardapi.PurchaseOrderEntity{ID: 1, VendorID: 10, Title: "By ID"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{
			getResult:    &boardapi.VendorEntity{ID: 10},
			searchResult: []boardapi.VendorEntity{{ID: 20}},
		},
		&stubPurchaseOrderRepo{
			getResult:    po,
			searchResult: []boardapi.PurchaseOrderEntity{{ID: 99, Title: "By Search"}},
		},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{ID: 1, VendorName: "ABC"})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 1)
	if got[0].PurchaseOrder.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].PurchaseOrder.ID)
	}
}

// --- FindPurchaseOrder: Limit Cases ---

func TestFindPurchaseOrder_Limit(t *testing.T) {
	pos := []boardapi.PurchaseOrderEntity{
		{ID: 1, VendorID: 10, Title: "PO1"},
		{ID: 2, VendorID: 10, Title: "PO2"},
		{ID: 3, VendorID: 10, Title: "PO3"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPurchaseOrderRepo{listResult: pos},
	)

	got, err := svc.FindPurchaseOrder(testCtx, find.FindPurchaseOrderQuery{Text: "PO", Limit: 2})
	assertNoError(t, err)
	assertPurchaseOrderResultLen(t, got, 2)
}
