package find_test

import (
	"errors"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// --- Constructor ---

func TestNew(t *testing.T) {
	svc := find.New(zeroRepos())
	if svc == nil {
		t.Fatal("New returned nil")
	}
}

// --- FindClient: Normal Cases ---

func TestFindClient_ByID(t *testing.T) {
	client := &boardapi.ClientEntity{ID: 1, Name: "Client A"}
	// M39: ClientID フィールド廃止 → Client nested 構造に変更。
	branches := []boardapi.ClientBranchEntity{
		{ID: 10, Client: &boardapi.ClientRef{ID: 1}, Name: "Branch 1"},
		{ID: 11, Client: &boardapi.ClientRef{ID: 1}, Name: "Branch 2"},
	}
	contacts := []boardapi.ContactEntity{
		{ID: 20, ClientID: 1, Name: "Contact 1"},
	}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		&stubClientBranchRepo{searchResult: branches},
		&stubContactRepo{searchResult: contacts},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{ID: 1})
	assertNoError(t, err)
	assertClientResultLen(t, got, 1)

	if got[0].Client.ID != 1 {
		t.Errorf("client ID = %d, want 1", got[0].Client.ID)
	}
	if len(got[0].Branches) != 2 {
		t.Errorf("branches len = %d, want 2", len(got[0].Branches))
	}
	if len(got[0].Contacts) != 1 {
		t.Errorf("contacts len = %d, want 1", len(got[0].Contacts))
	}
}

func TestFindClient_ByName(t *testing.T) {
	clients := []boardapi.ClientEntity{
		{ID: 1, Name: "ABC Corp"},
		{ID: 2, Name: "ABC Inc"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{Name: "ABC"})
	assertNoError(t, err)
	assertClientResultLen(t, got, 2)
}

func TestFindClient_ByText(t *testing.T) {
	allClients := []boardapi.ClientEntity{
		{ID: 1, Name: "Client A", Code: "CA", Memo: "important memo"},
		{ID: 2, Name: "Client B", Code: "CB", Memo: "normal"},
		{ID: 3, Name: "Client C", Code: "memo-code", Memo: ""},
	}

	svc := newServiceWith(
		&stubClientRepo{listResult: allClients},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{Text: "memo"})
	assertNoError(t, err)
	// Should match Client A (memo field) and Client C (code field)
	assertClientResultLen(t, got, 2)
}

func TestFindClient_EmptySubResources(t *testing.T) {
	client := &boardapi.ClientEntity{ID: 1, Name: "Lonely Client"}

	svc := newServiceWith(
		&stubClientRepo{getResult: client},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{ID: 1})
	assertNoError(t, err)
	assertClientResultLen(t, got, 1)

	if got[0].Branches != nil {
		t.Errorf("branches = %v, want nil", got[0].Branches)
	}
	if got[0].Contacts != nil {
		t.Errorf("contacts = %v, want nil", got[0].Contacts)
	}
}

// --- FindClient: Error/Edge Cases ---

func TestFindClient_NotFoundByID(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{err: errors.New("not found")},
		nil, nil, nil,
	)

	_, err := svc.FindClient(testCtx, find.FindClientQuery{ID: 999})
	assertError(t, err)
}

func TestFindClient_NoMatchByName(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: nil},
		nil, nil, nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{Name: "nonexistent"})
	assertNoError(t, err)
	assertClientResultLen(t, got, 0)
}

func TestFindClient_EmptyQuery(t *testing.T) {
	svc := find.New(zeroRepos())

	_, err := svc.FindClient(testCtx, find.FindClientQuery{})
	assertError(t, err)
}

func TestFindClient_RepoError(t *testing.T) {
	svc := newServiceWith(
		&stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 1, Name: "A"}}},
		&stubClientBranchRepo{err: errors.New("branch repo error")},
		nil, nil,
	)

	_, err := svc.FindClient(testCtx, find.FindClientQuery{Name: "A"})
	assertError(t, err)
}

// --- FindClient: Priority Cases ---

func TestFindClient_IDPriorityOverName(t *testing.T) {
	client := &boardapi.ClientEntity{ID: 1, Name: "By ID"}

	svc := newServiceWith(
		&stubClientRepo{
			getResult:    client,
			searchResult: []boardapi.ClientEntity{{ID: 2, Name: "By Name"}},
		},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{ID: 1, Name: "By Name"})
	assertNoError(t, err)
	assertClientResultLen(t, got, 1)
	if got[0].Client.ID != 1 {
		t.Errorf("expected ID lookup (1), got %d", got[0].Client.ID)
	}
}

func TestFindClient_NamePriorityOverText(t *testing.T) {
	searchResult := []boardapi.ClientEntity{{ID: 1, Name: "Search Result"}}

	svc := newServiceWith(
		&stubClientRepo{
			searchResult: searchResult,
			listResult:   []boardapi.ClientEntity{{ID: 2, Name: "List Result"}},
		},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{Name: "Search", Text: "List"})
	assertNoError(t, err)
	assertClientResultLen(t, got, 1)
	if got[0].Client.ID != 1 {
		t.Errorf("expected Name search (1), got %d", got[0].Client.ID)
	}
}

// --- FindClient: Limit Cases ---

func TestFindClient_Limit(t *testing.T) {
	clients := []boardapi.ClientEntity{
		{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"},
		{ID: 4, Name: "D"}, {ID: 5, Name: "E"},
	}

	svc := newServiceWith(
		&stubClientRepo{searchResult: clients},
		&stubClientBranchRepo{searchResult: nil},
		&stubContactRepo{searchResult: nil},
		nil,
	)

	got, err := svc.FindClient(testCtx, find.FindClientQuery{Name: "A", Limit: 2})
	assertNoError(t, err)
	assertClientResultLen(t, got, 2)
}
