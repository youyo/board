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

func assertEstimateResultLen(t *testing.T, got []find.EstimateResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("EstimateResult len = %d, want %d", len(got), want)
	}
}

func assertInvoiceResultLen(t *testing.T, got []find.InvoiceResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("InvoiceResult len = %d, want %d", len(got), want)
	}
}

func assertOrderResultLen(t *testing.T, got []find.OrderResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("OrderResult len = %d, want %d", len(got), want)
	}
}

func assertDeliveryResultLen(t *testing.T, got []find.DeliveryResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("DeliveryResult len = %d, want %d", len(got), want)
	}
}

func assertReceiptResultLen(t *testing.T, got []find.ReceiptResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("ReceiptResult len = %d, want %d", len(got), want)
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

type stubEstimateRepo struct {
	listResult   []boardapi.EstimateEntity
	getResult    *boardapi.EstimateEntity
	searchResult []boardapi.EstimateEntity
	err          error
}

func (s *stubEstimateRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.listResult, s.err
}
func (s *stubEstimateRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.getResult, s.err
}
func (s *stubEstimateRepo) Search(_ context.Context, _ boardapi.EstimateSearchParams, _ repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.searchResult, s.err
}

type stubInvoiceRepo struct {
	listResult   []boardapi.InvoiceEntity
	getResult    *boardapi.InvoiceEntity
	searchResult []boardapi.InvoiceEntity
	err          error
}

func (s *stubInvoiceRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.listResult, s.err
}
func (s *stubInvoiceRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	return s.getResult, s.err
}
func (s *stubInvoiceRepo) Search(_ context.Context, _ boardapi.InvoiceSearchParams, _ repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.searchResult, s.err
}

type stubOrderRepo struct {
	listResult   []boardapi.OrderEntity
	getResult    *boardapi.OrderEntity
	searchResult []boardapi.OrderEntity
	err          error
}

func (s *stubOrderRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.OrderEntity, error) {
	return s.listResult, s.err
}
func (s *stubOrderRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.OrderEntity, error) {
	return s.getResult, s.err
}
func (s *stubOrderRepo) Search(_ context.Context, _ boardapi.OrderSearchParams, _ repository.ReadOptions) ([]boardapi.OrderEntity, error) {
	return s.searchResult, s.err
}

type stubDeliveryRepo struct {
	listResult   []boardapi.DeliveryEntity
	getResult    *boardapi.DeliveryEntity
	searchResult []boardapi.DeliveryEntity
	err          error
}

func (s *stubDeliveryRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.listResult, s.err
}
func (s *stubDeliveryRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.getResult, s.err
}
func (s *stubDeliveryRepo) Search(_ context.Context, _ boardapi.DeliverySearchParams, _ repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.searchResult, s.err
}

type stubReceiptRepo struct {
	listResult   []boardapi.ReceiptEntity
	getResult    *boardapi.ReceiptEntity
	searchResult []boardapi.ReceiptEntity
	err          error
}

func (s *stubReceiptRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.listResult, s.err
}
func (s *stubReceiptRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.getResult, s.err
}
func (s *stubReceiptRepo) Search(_ context.Context, _ boardapi.ReceiptSearchParams, _ repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.searchResult, s.err
}

// --- Service constructors ---

func zeroRepos() find.Repos {
	return find.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
		Estimates:      &stubEstimateRepo{},
		Invoices:       &stubInvoiceRepo{},
		Orders:         &stubOrderRepo{},
		Deliveries:     &stubDeliveryRepo{},
		Receipts:       &stubReceiptRepo{},
	}
}

// newServiceWith creates a service with optional stub overrides.
// Pass nil for any repo to use the zero stub.
func newServiceWith(
	clients *stubClientRepo,
	branches *stubClientBranchRepo,
	contacts *stubContactRepo,
	projects *stubProjectRepo,
	opts ...interface{},
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
	// Process optional document repo stubs
	for _, opt := range opts {
		switch v := opt.(type) {
		case *stubEstimateRepo:
			r.Estimates = v
		case *stubInvoiceRepo:
			r.Invoices = v
		case *stubOrderRepo:
			r.Orders = v
		case *stubDeliveryRepo:
			r.Deliveries = v
		case *stubReceiptRepo:
			r.Receipts = v
		}
	}
	return find.New(r)
}
