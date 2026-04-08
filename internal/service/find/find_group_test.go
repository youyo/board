package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindGroup: Normal Cases ---

func TestFindGroup_ByID(t *testing.T) {
	group := &boardapi.GroupEntity{ID: 1, Name: "Engineering", Memo: "dev team"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{getResult: group},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{ID: 1})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 1)

	if got[0].Group.ID != 1 {
		t.Errorf("group ID = %d, want 1", got[0].Group.ID)
	}
	if got[0].Group.Name != "Engineering" {
		t.Errorf("group name = %q, want Engineering", got[0].Group.Name)
	}
}

func TestFindGroup_ByName(t *testing.T) {
	groups := []boardapi.GroupEntity{
		{ID: 1, Name: "Sales Team"},
		{ID: 2, Name: "Sales Support"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{searchResult: groups},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{Name: "Sales"})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 2)
}

func TestFindGroup_ByText(t *testing.T) {
	allGroups := []boardapi.GroupEntity{
		{ID: 1, Name: "Engineering", Memo: "development team"},
		{ID: 2, Name: "Sales", Memo: "revenue"},
		{ID: 3, Name: "DevOps", Memo: "infrastructure"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{listResult: allGroups},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{Text: "dev"})
	assertNoError(t, err)
	// Should match Engineering (memo: "development") and DevOps (name)
	assertGroupResultLen(t, got, 2)
}

// --- FindGroup: Error/Edge Cases ---

func TestFindGroup_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindGroup(testCtx, find.FindGroupQuery{})
	assertError(t, err)
}

func TestFindGroup_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{err: errors.New("not found")},
	)

	_, err := svc.FindGroup(testCtx, find.FindGroupQuery{ID: 999})
	assertError(t, err)
}

func TestFindGroup_NoMatchByName(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{searchResult: nil},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{Name: "nonexistent"})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 0)
}

// --- FindGroup: Priority Cases ---

func TestFindGroup_IDPriorityOverName(t *testing.T) {
	group := &boardapi.GroupEntity{ID: 1, Name: "By ID"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{
			getResult:    group,
			searchResult: []boardapi.GroupEntity{{ID: 2, Name: "By Name"}},
		},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{ID: 1, Name: "By Name"})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 1)
	if got[0].Group.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Group.ID)
	}
}

func TestFindGroup_NamePriorityOverText(t *testing.T) {
	searchResult := []boardapi.GroupEntity{{ID: 1, Name: "Search Result"}}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{
			searchResult: searchResult,
			listResult:   []boardapi.GroupEntity{{ID: 2, Name: "List Result"}},
		},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{Name: "Search", Text: "List"})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 1)
	if got[0].Group.ID != 1 {
		t.Errorf("expected Name search (1), got %d", got[0].Group.ID)
	}
}

// --- FindGroup: Limit Cases ---

func TestFindGroup_Limit(t *testing.T) {
	groups := []boardapi.GroupEntity{
		{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"},
		{ID: 4, Name: "D"}, {ID: 5, Name: "E"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubGroupRepo{searchResult: groups},
	)

	got, err := svc.FindGroup(testCtx, find.FindGroupQuery{Name: "A", Limit: 2})
	assertNoError(t, err)
	assertGroupResultLen(t, got, 2)
}
