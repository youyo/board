package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindPayment: Normal Cases ---

func TestFindPayment_ByID(t *testing.T) {
	payment := &boardapi.PaymentEntity{ID: 1, VendorID: 10, PurchaseOrderID: 100, Memo: "Payment A"}
	vendor := &boardapi.VendorEntity{ID: 10, Name: "Vendor X"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: vendor},
		&stubPaymentRepo{getResult: payment},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{ID: 1})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)

	if got[0].Payment.ID != 1 {
		t.Errorf("payment ID = %d, want 1", got[0].Payment.ID)
	}
	if got[0].Vendor == nil || got[0].Vendor.ID != 10 {
		t.Errorf("vendor not resolved correctly")
	}
}

func TestFindPayment_ByVendorName(t *testing.T) {
	vendors := []boardapi.VendorEntity{{ID: 10, Name: "ABC Corp"}}
	payments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, Memo: "P1"},
		{ID: 2, VendorID: 10, Memo: "P2"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors, getResult: &boardapi.VendorEntity{ID: 10, Name: "ABC Corp"}},
		&stubPaymentRepo{searchResult: payments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{VendorName: "ABC"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 2)
}

func TestFindPayment_ByPurchaseOrderID(t *testing.T) {
	payments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, PurchaseOrderID: 100, Memo: "P1"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPaymentRepo{searchResult: payments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{PurchaseOrderID: 100})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)
	if got[0].Payment.PurchaseOrderID != 100 {
		t.Errorf("purchase order ID = %d, want 100", got[0].Payment.PurchaseOrderID)
	}
}

func TestFindPayment_ByText(t *testing.T) {
	allPayments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, Memo: "important payment"},
		{ID: 2, VendorID: 10, Memo: "normal"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPaymentRepo{listResult: allPayments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{Text: "important"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)
	if got[0].Payment.ID != 1 {
		t.Errorf("payment ID = %d, want 1", got[0].Payment.ID)
	}
}

func TestFindPayment_ByStatus(t *testing.T) {
	allPayments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, Status: "paid"},
		{ID: 2, VendorID: 10, Status: "pending"},
		{ID: 3, VendorID: 10, Status: "paid"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPaymentRepo{listResult: allPayments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{Status: "paid"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 2)
}

func TestFindPayment_ByVendorNameWithStatus(t *testing.T) {
	vendors := []boardapi.VendorEntity{{ID: 10, Name: "ABC"}}
	payments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, Status: "paid", Memo: "P1"},
		{ID: 2, VendorID: 10, Status: "pending", Memo: "P2"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors, getResult: &boardapi.VendorEntity{ID: 10, Name: "ABC"}},
		&stubPaymentRepo{searchResult: payments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{VendorName: "ABC", Status: "paid"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)
	if got[0].Payment.Status != "paid" {
		t.Errorf("status = %q, want paid", got[0].Payment.Status)
	}
}

// --- FindPayment: Error/Edge Cases ---

func TestFindPayment_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindPayment(testCtx, find.FindPaymentQuery{})
	assertError(t, err)
}

func TestFindPayment_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubPaymentRepo{err: errors.New("not found")},
	)

	_, err := svc.FindPayment(testCtx, find.FindPaymentQuery{ID: 999})
	assertError(t, err)
}

func TestFindPayment_NoMatchByVendorName(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: nil},
		&stubPaymentRepo{},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{VendorName: "nonexistent"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 0)
}

func TestFindPayment_VendorResolutionFailure(t *testing.T) {
	payment := &boardapi.PaymentEntity{ID: 1, VendorID: 10}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{err: errors.New("vendor error")},
		&stubPaymentRepo{getResult: payment},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{ID: 1})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)
	if got[0].Vendor != nil {
		t.Error("expected nil vendor on resolution failure")
	}
}

// --- FindPayment: Priority Cases ---

func TestFindPayment_IDPriorityOverVendorName(t *testing.T) {
	payment := &boardapi.PaymentEntity{ID: 1, VendorID: 10, Memo: "By ID"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{
			getResult:    &boardapi.VendorEntity{ID: 10},
			searchResult: []boardapi.VendorEntity{{ID: 20}},
		},
		&stubPaymentRepo{
			getResult:    payment,
			searchResult: []boardapi.PaymentEntity{{ID: 99, Memo: "By Search"}},
		},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{ID: 1, VendorName: "ABC"})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 1)
	if got[0].Payment.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Payment.ID)
	}
}

// --- FindPayment: Limit Cases ---

func TestFindPayment_Limit(t *testing.T) {
	payments := []boardapi.PaymentEntity{
		{ID: 1, VendorID: 10, Memo: "P1"},
		{ID: 2, VendorID: 10, Memo: "P2"},
		{ID: 3, VendorID: 10, Memo: "P3"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: &boardapi.VendorEntity{ID: 10}},
		&stubPaymentRepo{listResult: payments},
	)

	got, err := svc.FindPayment(testCtx, find.FindPaymentQuery{Text: "P", Limit: 2})
	assertNoError(t, err)
	assertPaymentResultLen(t, got, 2)
}
