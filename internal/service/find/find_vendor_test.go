package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindVendor: Normal Cases ---

func TestFindVendor_ByID(t *testing.T) {
	vendor := &boardapi.VendorEntity{ID: 1, Name: "Vendor A"}
	branches := []boardapi.VendorBranchEntity{
		{ID: 10, Vendor: &boardapi.VendorRef{ID: 1}, Name: "Branch 1"},
		{ID: 11, Vendor: &boardapi.VendorRef{ID: 1}, Name: "Branch 2"},
	}
	contacts := []boardapi.VendorContactEntity{
		{ID: 20, VendorID: 1, Name: "Contact 1"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: vendor},
		&stubVendorBranchRepo{searchResult: branches},
		&stubVendorContactRepo{searchResult: contacts},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{ID: 1})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 1)

	if got[0].Vendor.ID != 1 {
		t.Errorf("vendor ID = %d, want 1", got[0].Vendor.ID)
	}
	if len(got[0].Branches) != 2 {
		t.Errorf("branches len = %d, want 2", len(got[0].Branches))
	}
	if len(got[0].Contacts) != 1 {
		t.Errorf("contacts len = %d, want 1", len(got[0].Contacts))
	}
}

func TestFindVendor_ByName(t *testing.T) {
	vendors := []boardapi.VendorEntity{
		{ID: 1, Name: "ABC Corp"},
		{ID: 2, Name: "ABC Inc"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{Name: "ABC"})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 2)
}

func TestFindVendor_ByText(t *testing.T) {
	allVendors := []boardapi.VendorEntity{
		{ID: 1, Name: "Vendor A", Code: "VA", Memo: "important memo"},
		{ID: 2, Name: "Vendor B", Code: "VB", Memo: "normal"},
		{ID: 3, Name: "Vendor C", Code: "memo-code", Memo: ""},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{listResult: allVendors},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{Text: "memo"})
	assertNoError(t, err)
	// Should match Vendor A (memo field) and Vendor C (code field)
	assertVendorResultLen(t, got, 2)
}

func TestFindVendor_EmptySubResources(t *testing.T) {
	vendor := &boardapi.VendorEntity{ID: 1, Name: "Lonely Vendor"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{getResult: vendor},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{ID: 1})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 1)

	if got[0].Branches != nil {
		t.Errorf("branches = %v, want nil", got[0].Branches)
	}
	if got[0].Contacts != nil {
		t.Errorf("contacts = %v, want nil", got[0].Contacts)
	}
}

// --- FindVendor: Error/Edge Cases ---

func TestFindVendor_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{err: errors.New("not found")},
	)

	_, err := svc.FindVendor(testCtx, find.FindVendorQuery{ID: 999})
	assertError(t, err)
}

func TestFindVendor_NoMatchByName(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{Name: "nonexistent"})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 0)
}

func TestFindVendor_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())

	_, err := svc.FindVendor(testCtx, find.FindVendorQuery{})
	assertError(t, err)
}

func TestFindVendor_RepoError(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: []boardapi.VendorEntity{{ID: 1, Name: "A"}}},
		&stubVendorBranchRepo{err: errors.New("branch repo error")},
	)

	_, err := svc.FindVendor(testCtx, find.FindVendorQuery{Name: "A"})
	assertError(t, err)
}

// --- FindVendor: Priority Cases ---

func TestFindVendor_IDPriorityOverName(t *testing.T) {
	vendor := &boardapi.VendorEntity{ID: 1, Name: "By ID"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{
			getResult:    vendor,
			searchResult: []boardapi.VendorEntity{{ID: 2, Name: "By Name"}},
		},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{ID: 1, Name: "By Name"})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 1)
	if got[0].Vendor.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Vendor.ID)
	}
}

func TestFindVendor_NamePriorityOverText(t *testing.T) {
	searchResult := []boardapi.VendorEntity{{ID: 1, Name: "Search Result"}}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{
			searchResult: searchResult,
			listResult:   []boardapi.VendorEntity{{ID: 2, Name: "List Result"}},
		},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{Name: "Search", Text: "List"})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 1)
	if got[0].Vendor.ID != 1 {
		t.Errorf("expected Name search (1), got %d", got[0].Vendor.ID)
	}
}

// --- FindVendor: Limit Cases ---

func TestFindVendor_Limit(t *testing.T) {
	vendors := []boardapi.VendorEntity{
		{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"},
		{ID: 4, Name: "D"}, {ID: 5, Name: "E"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubVendorRepo{searchResult: vendors},
		&stubVendorBranchRepo{searchResult: nil},
		&stubVendorContactRepo{searchResult: nil},
	)

	got, err := svc.FindVendor(testCtx, find.FindVendorQuery{Name: "A", Limit: 2})
	assertNoError(t, err)
	assertVendorResultLen(t, got, 2)
}
