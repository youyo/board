package find2

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// newUserTestService は FindUser テスト用の Service を生成するヘルパー。
func newUserTestService(users *stubUserRepo) *Service {
	r := newTestRepos()
	r.Users = users
	return New(r)
}

// U01: ByID HappyPath
func TestService_FindUser_ByID_HappyPath(t *testing.T) {
	users := &stubUserRepo{
		getResult: &boardapi.UserEntity{ID: 10, Name: "Alice", Email: "alice@example.com"},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{ID: 10})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].User.ID != 10 || results[0].User.Name != "Alice" {
		t.Errorf("user mismatch: %+v", results[0].User)
	}
}

// U02: ByName — NameCont が API に渡る
func TestService_FindUser_ByName_DelegatesNameCont(t *testing.T) {
	var captured boardapi.UserListOptions
	users := &stubUserRepo{
		searchFunc: func(_ context.Context, f boardapi.UserListOptions, _ repository.ReadOptions) ([]boardapi.UserEntity, error) {
			captured = f
			return []boardapi.UserEntity{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Aliciana"}}, nil
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{Name: "Ali"})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if captured.NameCont != "Ali" {
		t.Errorf("NameCont=%q, want 'Ali'", captured.NameCont)
	}
}

// U03: ByText — Name マッチ
func TestService_FindUser_ByText_MatchesName(t *testing.T) {
	users := &stubUserRepo{
		searchResult: []boardapi.UserEntity{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{Text: "alice"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// U04: ByText — LastName マッチ
func TestService_FindUser_ByText_MatchesLastName(t *testing.T) {
	users := &stubUserRepo{
		searchResult: []boardapi.UserEntity{
			{ID: 1, LastName: "Tanaka"},
			{ID: 2, LastName: "Suzuki"},
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{Text: "tanaka"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// U05: ByText — FirstName マッチ
func TestService_FindUser_ByText_MatchesFirstName(t *testing.T) {
	users := &stubUserRepo{
		searchResult: []boardapi.UserEntity{
			{ID: 1, FirstName: "Taro"},
			{ID: 2, FirstName: "Jiro"},
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{Text: "taro"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// U06: ByText — Email マッチ
func TestService_FindUser_ByText_MatchesEmail(t *testing.T) {
	users := &stubUserRepo{
		searchResult: []boardapi.UserEntity{
			{ID: 1, Email: "alice@example.com"},
			{ID: 2, Email: "bob@example.com"},
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{Text: "alice@"})
	assertNoError(t, err)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// U07: Limit=2 — 3 件中 2 件で打ち切り
func TestService_FindUser_LimitTwo_StopsLoop(t *testing.T) {
	users := &stubUserRepo{
		searchResult: []boardapi.UserEntity{
			{ID: 1, Name: "user-a"},
			{ID: 2, Name: "user-b"},
			{ID: 3, Name: "user-c"},
		},
	}
	svc := newUserTestService(users)

	results, err := svc.FindUser(testCtx, FindUserQuery{
		Text:           "user",
		FindCommonOpts: FindCommonOpts{Limit: 2},
	})
	assertNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// U08: Empty query → error
func TestService_FindUser_EmptyQuery_Error(t *testing.T) {
	svc := newUserTestService(&stubUserRepo{})

	_, err := svc.FindUser(testCtx, FindUserQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one field required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// U09: GetByID error → fail-fast
func TestService_FindUser_GetByIDError_Bubbles(t *testing.T) {
	fakeErr := errors.New("user API error")
	users := &stubUserRepo{err: fakeErr}
	svc := newUserTestService(users)

	_, err := svc.FindUser(testCtx, FindUserQuery{ID: 10})
	if !errors.Is(err, fakeErr) {
		t.Errorf("expected fakeErr, got %v", err)
	}
}

// U10: PriorityIDOverridesName — ID が優先されると Search は呼ばれない
func TestService_FindUser_Priority_IDOverridesName(t *testing.T) {
	users := &stubUserRepo{
		getResult: &boardapi.UserEntity{ID: 10, Name: "Alice"},
	}
	svc := newUserTestService(users)

	_, err := svc.FindUser(testCtx, FindUserQuery{ID: 10, Name: "Alice"})
	assertNoError(t, err)
	if users.searchCount != 0 {
		t.Errorf("Search should not be called when ID is set, got %d", users.searchCount)
	}
}

// U11: LimitNegative → error
func TestService_FindUser_LimitNegative_Error(t *testing.T) {
	svc := newUserTestService(&stubUserRepo{})

	_, err := svc.FindUser(testCtx, FindUserQuery{
		ID:             10,
		FindCommonOpts: FindCommonOpts{Limit: -1},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "limit must be >= 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}
