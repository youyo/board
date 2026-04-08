package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- FindUser: Normal Cases ---

func TestFindUser_ByID(t *testing.T) {
	user := &boardapi.UserEntity{ID: 1, Name: "User A", Email: "a@example.com"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{getResult: user},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{ID: 1})
	assertNoError(t, err)
	assertUserResultLen(t, got, 1)

	if got[0].User.ID != 1 {
		t.Errorf("user ID = %d, want 1", got[0].User.ID)
	}
	if got[0].User.Name != "User A" {
		t.Errorf("user name = %q, want User A", got[0].User.Name)
	}
}

func TestFindUser_ByName(t *testing.T) {
	users := []boardapi.UserEntity{
		{ID: 1, Name: "Alice Smith"},
		{ID: 2, Name: "Alice Jones"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{searchResult: users},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{Name: "Alice"})
	assertNoError(t, err)
	assertUserResultLen(t, got, 2)
}

func TestFindUser_ByText(t *testing.T) {
	allUsers := []boardapi.UserEntity{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Name: "Charlie", Email: "alice-friend@example.com"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{listResult: allUsers},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{Text: "alice"})
	assertNoError(t, err)
	// Should match User 1 (name) and User 3 (email)
	assertUserResultLen(t, got, 2)
}

// --- FindUser: Error/Edge Cases ---

func TestFindUser_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())
	_, err := svc.FindUser(testCtx, find.FindUserQuery{})
	assertError(t, err)
}

func TestFindUser_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{err: errors.New("not found")},
	)

	_, err := svc.FindUser(testCtx, find.FindUserQuery{ID: 999})
	assertError(t, err)
}

func TestFindUser_NoMatchByName(t *testing.T) {
	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{searchResult: nil},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{Name: "nonexistent"})
	assertNoError(t, err)
	assertUserResultLen(t, got, 0)
}

// --- FindUser: Priority Cases ---

func TestFindUser_IDPriorityOverName(t *testing.T) {
	user := &boardapi.UserEntity{ID: 1, Name: "By ID"}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{
			getResult:    user,
			searchResult: []boardapi.UserEntity{{ID: 2, Name: "By Name"}},
		},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{ID: 1, Name: "By Name"})
	assertNoError(t, err)
	assertUserResultLen(t, got, 1)
	if got[0].User.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].User.ID)
	}
}

func TestFindUser_NamePriorityOverText(t *testing.T) {
	searchResult := []boardapi.UserEntity{{ID: 1, Name: "Search Result"}}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{
			searchResult: searchResult,
			listResult:   []boardapi.UserEntity{{ID: 2, Name: "List Result"}},
		},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{Name: "Search", Text: "List"})
	assertNoError(t, err)
	assertUserResultLen(t, got, 1)
	if got[0].User.ID != 1 {
		t.Errorf("expected Name search (1), got %d", got[0].User.ID)
	}
}

// --- FindUser: Limit Cases ---

func TestFindUser_Limit(t *testing.T) {
	users := []boardapi.UserEntity{
		{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"},
		{ID: 4, Name: "D"}, {ID: 5, Name: "E"},
	}

	svc := newServiceWith(
		nil, nil, nil, nil,
		&stubUserRepo{searchResult: users},
	)

	got, err := svc.FindUser(testCtx, find.FindUserQuery{Name: "A", Limit: 2})
	assertNoError(t, err)
	assertUserResultLen(t, got, 2)
}
