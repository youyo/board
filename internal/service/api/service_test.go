package api_test

import (
	"context"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// --- テスト共通ヘルパー ---

var testCtx = context.Background()
var defaultOpts = repository.ReadOptions{}

// assertNoError はエラーがないことを確認する。
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertLen はスライスの長さを確認する。
func assertLen[T any](t *testing.T, got []T, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("len = %d, want %d", len(got), want)
	}
}

// assertNotNil はポインタが nil でないことを確認する。
func assertNotNil[T any](t *testing.T, got *T) {
	t.Helper()
	if got == nil {
		t.Fatal("got nil, want non-nil")
	}
}

// stubClientRepo は ClientRepository のスタブ実装。
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

// stubClientBranchRepo は ClientBranchRepository のスタブ実装。
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

// stubContactRepo は ContactRepository のスタブ実装。
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

// stubProjectRepo は ProjectRepository のスタブ実装。
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

// stubProjectCostRepo は ProjectCostRepository のスタブ実装。
type stubProjectCostRepo struct {
	listResult   []boardapi.ProjectCostEntity
	getResult    *boardapi.ProjectCostEntity
	searchResult []boardapi.ProjectCostEntity
	err          error
}

func (s *stubProjectCostRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.listResult, s.err
}
func (s *stubProjectCostRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ProjectCostEntity, error) {
	return s.getResult, s.err
}
func (s *stubProjectCostRepo) Search(_ context.Context, _ boardapi.ProjectCostSearchParams, _ repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.searchResult, s.err
}

// --- Clients テスト ---

func TestListClients(t *testing.T) {
	stub := &stubClientRepo{listResult: []boardapi.ClientEntity{{ID: 1, Name: "顧客A"}}}
	svc := newServiceWithClients(stub)
	got, err := svc.ListClients(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetClient(t *testing.T) {
	entity := &boardapi.ClientEntity{ID: 1, Name: "顧客A"}
	stub := &stubClientRepo{getResult: entity}
	svc := newServiceWithClients(stub)
	got, err := svc.GetClient(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchClients(t *testing.T) {
	stub := &stubClientRepo{searchResult: []boardapi.ClientEntity{{ID: 2, Name: "顧客B"}}}
	svc := newServiceWithClients(stub)
	got, err := svc.SearchClients(testCtx, boardapi.ClientSearchParams{Name: "顧客B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- ClientBranches テスト ---

func TestListClientBranches(t *testing.T) {
	stub := &stubClientBranchRepo{listResult: []boardapi.ClientBranchEntity{{ID: 1, Name: "支社A"}}}
	svc := newServiceWithClientBranches(stub)
	got, err := svc.ListClientBranches(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetClientBranch(t *testing.T) {
	entity := &boardapi.ClientBranchEntity{ID: 1, Name: "支社A"}
	stub := &stubClientBranchRepo{getResult: entity}
	svc := newServiceWithClientBranches(stub)
	got, err := svc.GetClientBranch(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchClientBranches(t *testing.T) {
	stub := &stubClientBranchRepo{searchResult: []boardapi.ClientBranchEntity{{ID: 2, Name: "支社B"}}}
	svc := newServiceWithClientBranches(stub)
	got, err := svc.SearchClientBranches(testCtx, boardapi.ClientBranchSearchParams{ClientID: 1}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Contacts テスト ---

func TestListContacts(t *testing.T) {
	stub := &stubContactRepo{listResult: []boardapi.ContactEntity{{ID: 1, Name: "担当者A"}}}
	svc := newServiceWithContacts(stub)
	got, err := svc.ListContacts(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetContact(t *testing.T) {
	entity := &boardapi.ContactEntity{ID: 1, Name: "担当者A"}
	stub := &stubContactRepo{getResult: entity}
	svc := newServiceWithContacts(stub)
	got, err := svc.GetContact(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchContacts(t *testing.T) {
	stub := &stubContactRepo{searchResult: []boardapi.ContactEntity{{ID: 2, Name: "担当者B"}}}
	svc := newServiceWithContacts(stub)
	got, err := svc.SearchContacts(testCtx, boardapi.ContactSearchParams{Name: "担当者B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Projects テスト ---

func TestListProjects(t *testing.T) {
	stub := &stubProjectRepo{listResult: []boardapi.ProjectEntity{{ID: 1, Name: "案件A"}}}
	svc := newServiceWithProjects(stub)
	got, err := svc.ListProjects(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetProject(t *testing.T) {
	entity := &boardapi.ProjectEntity{ID: 1, Name: "案件A"}
	stub := &stubProjectRepo{getResult: entity}
	svc := newServiceWithProjects(stub)
	got, err := svc.GetProject(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchProjects(t *testing.T) {
	stub := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{{ID: 2, Name: "案件B"}}}
	svc := newServiceWithProjects(stub)
	got, err := svc.SearchProjects(testCtx, boardapi.ProjectSearchParams{Name: "案件B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- ProjectCosts テスト ---

func TestListProjectCosts(t *testing.T) {
	stub := &stubProjectCostRepo{listResult: []boardapi.ProjectCostEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithProjectCosts(stub)
	got, err := svc.ListProjectCosts(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetProjectCost(t *testing.T) {
	entity := &boardapi.ProjectCostEntity{ID: 1, ProjectID: 10}
	stub := &stubProjectCostRepo{getResult: entity}
	svc := newServiceWithProjectCosts(stub)
	got, err := svc.GetProjectCost(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchProjectCosts(t *testing.T) {
	stub := &stubProjectCostRepo{searchResult: []boardapi.ProjectCostEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithProjectCosts(stub)
	got, err := svc.SearchProjectCosts(testCtx, boardapi.ProjectCostSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}
