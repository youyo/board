package find_test

import (
	"context"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// --- Test context and options ---

var testCtx = context.Background()

// --- Assertion helpers ---

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func assertClientResultLen(t *testing.T, got []find.ClientResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("ClientResult len = %d, want %d", len(got), want)
	}
}

func assertProjectResultLen(t *testing.T, got []find.ProjectResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("ProjectResult len = %d, want %d", len(got), want)
	}
}

// --- Stub implementations ---

type stubClientRepo struct {
	listResult   []boardapi.ClientEntity
	getResult    *boardapi.ClientEntity
	searchResult []boardapi.ClientEntity
	err          error
}

func (s *stubClientRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.listResult, s.err
}
func (s *stubClientRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ClientEntity, error) {
	return s.getResult, s.err
}
func (s *stubClientRepo) Search(_ context.Context, _ boardapi.ClientSearchParams, _ repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.searchResult, s.err
}

type stubClientBranchRepo struct {
	listResult   []boardapi.ClientBranchEntity
	getResult    *boardapi.ClientBranchEntity
	searchResult []boardapi.ClientBranchEntity
	err          error
}

func (s *stubClientBranchRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.listResult, s.err
}
func (s *stubClientBranchRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ClientBranchEntity, error) {
	return s.getResult, s.err
}
func (s *stubClientBranchRepo) Search(_ context.Context, _ boardapi.ClientBranchSearchParams, _ repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.searchResult, s.err
}

type stubContactRepo struct {
	listResult   []boardapi.ContactEntity
	getResult    *boardapi.ContactEntity
	searchResult []boardapi.ContactEntity
	err          error
}

func (s *stubContactRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.listResult, s.err
}
func (s *stubContactRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ContactEntity, error) {
	return s.getResult, s.err
}
func (s *stubContactRepo) Search(_ context.Context, _ boardapi.ContactSearchParams, _ repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.searchResult, s.err
}

type stubProjectRepo struct {
	listResult   []boardapi.ProjectEntity
	getResult    *boardapi.ProjectEntity
	searchResult []boardapi.ProjectEntity
	err          error
}

func (s *stubProjectRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.listResult, s.err
}
func (s *stubProjectRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	return s.getResult, s.err
}
func (s *stubProjectRepo) Search(_ context.Context, _ boardapi.ProjectSearchParams, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.searchResult, s.err
}

// --- Service constructors ---

func zeroRepos() find.Repos {
	return find.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
	}
}

func newServiceWith(
	clients *stubClientRepo,
	branches *stubClientBranchRepo,
	contacts *stubContactRepo,
	projects *stubProjectRepo,
) *find.Service {
	r := zeroRepos()
	if clients != nil {
		r.Clients = clients
	}
	if branches != nil {
		r.ClientBranches = branches
	}
	if contacts != nil {
		r.Contacts = contacts
	}
	if projects != nil {
		r.Projects = projects
	}
	return find.New(r)
}
