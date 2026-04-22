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

// strPtr は文字列を *string に変換するテストヘルパー。
func strPtr(s string) *string { return &s }

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

func assertVendorResultLen(t *testing.T, got []find.VendorResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("VendorResult len = %d, want %d", len(got), want)
	}
}

func assertPurchaseOrderResultLen(t *testing.T, got []find.PurchaseOrderResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("PurchaseOrderResult len = %d, want %d", len(got), want)
	}
}

func assertPaymentResultLen(t *testing.T, got []find.PaymentResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("PaymentResult len = %d, want %d", len(got), want)
	}
}

func assertUserResultLen(t *testing.T, got []find.UserResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("UserResult len = %d, want %d", len(got), want)
	}
}

func assertGroupResultLen(t *testing.T, got []find.GroupResult, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("GroupResult len = %d, want %d", len(got), want)
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
	listResult         []boardapi.ProjectEntity
	getResult          *boardapi.ProjectEntity
	searchResult       []boardapi.ProjectEntity
	getWithGroupResult *boardapi.ProjectEntity
	err                error
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
func (s *stubProjectRepo) GetByIDWithGroup(_ context.Context, id int, _ string) (*boardapi.ProjectEntity, error) {
	if s.getWithGroupResult != nil {
		return s.getWithGroupResult, nil
	}
	if s.getResult != nil {
		return s.getResult, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	// Return a bare entity with no document summary so enrichment is a no-op
	return &boardapi.ProjectEntity{ID: id}, nil
}

type stubEstimateRepo struct {
	getByDocIDResult *boardapi.EstimateEntity
	err              error
}

func (s *stubEstimateRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.getByDocIDResult, s.err
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
	getByDocIDResult *boardapi.OrderEntity
	err              error
}

func (s *stubOrderRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.OrderEntity, error) {
	return s.getByDocIDResult, s.err
}

type stubDeliveryRepo struct {
	getByDocIDResult *boardapi.DeliveryEntity
	err              error
}

func (s *stubDeliveryRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.getByDocIDResult, s.err
}

type stubReceiptRepo struct {
	getByDocIDResult *boardapi.ReceiptEntity
	err              error
}

func (s *stubReceiptRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.getByDocIDResult, s.err
}

type stubVendorRepo struct {
	listResult   []boardapi.VendorEntity
	getResult    *boardapi.VendorEntity
	searchResult []boardapi.VendorEntity
	err          error
}

func (s *stubVendorRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.listResult, s.err
}
func (s *stubVendorRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.VendorEntity, error) {
	return s.getResult, s.err
}
func (s *stubVendorRepo) Search(_ context.Context, _ boardapi.VendorSearchParams, _ repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.searchResult, s.err
}

type stubVendorBranchRepo struct {
	listResult   []boardapi.VendorBranchEntity
	getResult    *boardapi.VendorBranchEntity
	searchResult []boardapi.VendorBranchEntity
	err          error
}

func (s *stubVendorBranchRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.listResult, s.err
}
func (s *stubVendorBranchRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.VendorBranchEntity, error) {
	return s.getResult, s.err
}
func (s *stubVendorBranchRepo) Search(_ context.Context, _ boardapi.VendorBranchSearchParams, _ repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.searchResult, s.err
}

type stubVendorContactRepo struct {
	listResult   []boardapi.VendorContactEntity
	getResult    *boardapi.VendorContactEntity
	searchResult []boardapi.VendorContactEntity
	err          error
}

func (s *stubVendorContactRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.listResult, s.err
}
func (s *stubVendorContactRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.VendorContactEntity, error) {
	return s.getResult, s.err
}
func (s *stubVendorContactRepo) Search(_ context.Context, _ boardapi.VendorContactSearchParams, _ repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.searchResult, s.err
}

type stubPurchaseOrderRepo struct {
	listResult   []boardapi.PurchaseOrderEntity
	getResult    *boardapi.PurchaseOrderEntity
	searchResult []boardapi.PurchaseOrderEntity
	err          error
}

func (s *stubPurchaseOrderRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.listResult, s.err
}
func (s *stubPurchaseOrderRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	return s.getResult, s.err
}
func (s *stubPurchaseOrderRepo) Search(_ context.Context, _ boardapi.PurchaseOrderSearchParams, _ repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.searchResult, s.err
}

type stubPaymentRepo struct {
	listResult   []boardapi.PaymentEntity
	getResult    *boardapi.PaymentEntity
	searchResult []boardapi.PaymentEntity
	err          error
}

func (s *stubPaymentRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.listResult, s.err
}
func (s *stubPaymentRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	return s.getResult, s.err
}
func (s *stubPaymentRepo) Search(_ context.Context, _ boardapi.PaymentSearchParams, _ repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.searchResult, s.err
}

type stubUserRepo struct {
	listResult   []boardapi.UserEntity
	getResult    *boardapi.UserEntity
	searchResult []boardapi.UserEntity
	err          error
}

func (s *stubUserRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.listResult, s.err
}
func (s *stubUserRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.UserEntity, error) {
	return s.getResult, s.err
}
func (s *stubUserRepo) Search(_ context.Context, _ boardapi.UserSearchParams, _ repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.searchResult, s.err
}

type stubGroupRepo struct {
	listResult   []boardapi.GroupEntity
	getResult    *boardapi.GroupEntity
	searchResult []boardapi.GroupEntity
	err          error
}

func (s *stubGroupRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.GroupEntity, error) {
	return s.listResult, s.err
}
func (s *stubGroupRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.GroupEntity, error) {
	return s.getResult, s.err
}
func (s *stubGroupRepo) Search(_ context.Context, _ boardapi.GroupSearchParams, _ repository.ReadOptions) ([]boardapi.GroupEntity, error) {
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
		Vendors:        &stubVendorRepo{},
		VendorBranches: &stubVendorBranchRepo{},
		VendorContacts: &stubVendorContactRepo{},
		PurchaseOrders: &stubPurchaseOrderRepo{},
		Payments:       &stubPaymentRepo{},
		Users:          &stubUserRepo{},
		Groups:         &stubGroupRepo{},
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
	// Process optional repo stubs
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
		case *stubVendorRepo:
			r.Vendors = v
		case *stubVendorBranchRepo:
			r.VendorBranches = v
		case *stubVendorContactRepo:
			r.VendorContacts = v
		case *stubPurchaseOrderRepo:
			r.PurchaseOrders = v
		case *stubPaymentRepo:
			r.Payments = v
		case *stubUserRepo:
			r.Users = v
		case *stubGroupRepo:
			r.Groups = v
		}
	}
	return find.New(r)
}
