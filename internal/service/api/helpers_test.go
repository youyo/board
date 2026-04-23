package api_test

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
	svcapi "github.com/youyo/board/internal/service/api"
)

// --- Stubs: core ---

// stubClientRepo is a stub implementation of ClientRepo.
// M50 以降: Search / ListPage は削除、List は (readOpts, filter) 二引数化。
// 非ゼロ filter 時に searchResult を返し、ゼロ filter 時は listResult を返す
// （repository の cache-bypass / cache-backed 分岐を模倣）。
type stubClientRepo struct {
	listResult   []boardapi.ClientEntity
	getResult    *boardapi.ClientEntity
	searchResult []boardapi.ClientEntity
	meta         boardapi.ListMeta
	err          error
}

func (s *stubClientRepo) List(_ context.Context, _ repository.ReadOptions, filter boardapi.ClientListOptions) (*boardapi.ListResult[boardapi.ClientEntity], error) {
	if s.err != nil {
		return nil, s.err
	}
	items := s.listResult
	if filter.NameCont != "" || filter.CustomNoEq != "" || len(filter.Tags) != 0 ||
		filter.NameDispCont != "" || filter.InvoiceSystemNumberEq != "" ||
		filter.UpdatedAtGteq != "" || filter.UpdatedAtLteq != "" ||
		filter.IncludeArchiveFlg != nil || filter.ResponseGroup != "" {
		items = s.searchResult
	}
	return &boardapi.ListResult[boardapi.ClientEntity]{Items: items, Meta: s.meta}, nil
}
func (s *stubClientRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ClientEntity, error) {
	return s.getResult, s.err
}

// stubClientBranchRepo is a stub implementation of ClientBranchRepo.
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
type stubClientBranchRepo struct {
	listResult []boardapi.ClientBranchEntity
	getResult  *boardapi.ClientBranchEntity
	meta       boardapi.ListMeta
	err        error
}

func (s *stubClientBranchRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.ClientBranchListOptions) (*boardapi.ListResult[boardapi.ClientBranchEntity], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &boardapi.ListResult[boardapi.ClientBranchEntity]{Items: s.listResult, Meta: s.meta}, nil
}
func (s *stubClientBranchRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ClientBranchEntity, error) {
	return s.getResult, s.err
}

// stubContactRepo is a stub implementation of ContactRepo.
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
type stubContactRepo struct {
	listResult []boardapi.ContactEntity
	getResult  *boardapi.ContactEntity
	meta       boardapi.ListMeta
	err        error
}

func (s *stubContactRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.ContactListOptions) (*boardapi.ListResult[boardapi.ContactEntity], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &boardapi.ListResult[boardapi.ContactEntity]{Items: s.listResult, Meta: s.meta}, nil
}
func (s *stubContactRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ContactEntity, error) {
	return s.getResult, s.err
}

// stubProjectRepo is a stub implementation of ProjectRepo.
type stubProjectRepo struct {
	listResult         []boardapi.ProjectEntity
	getResult          *boardapi.ProjectEntity
	getWithGroupResult *boardapi.ProjectEntity
	err                error
}

func (s *stubProjectRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.ProjectListOptions) (*boardapi.ListResult[boardapi.ProjectEntity], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &boardapi.ListResult[boardapi.ProjectEntity]{Items: s.listResult}, nil
}
func (s *stubProjectRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	return s.getResult, s.err
}
func (s *stubProjectRepo) GetByIDWithGroup(_ context.Context, _ int, _ string) (*boardapi.ProjectEntity, error) {
	return s.getWithGroupResult, s.err
}

// stubProjectCostRepo is a stub implementation of ProjectCostRepo.
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
type stubProjectCostRepo struct {
	listResult []boardapi.ProjectCostEntity
	getResult  *boardapi.ProjectCostEntity
	meta       boardapi.ListMeta
	err        error
}

func (s *stubProjectCostRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.ProjectCostListOptions) (*boardapi.ListResult[boardapi.ProjectCostEntity], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &boardapi.ListResult[boardapi.ProjectCostEntity]{Items: s.listResult, Meta: s.meta}, nil
}
func (s *stubProjectCostRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ProjectCostEntity, error) {
	return s.getResult, s.err
}

// --- Stubs: document ---

type stubEstimateRepo struct {
	getResult *boardapi.EstimateEntity
	err       error
}

func (s *stubEstimateRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.getResult, s.err
}

type stubInvoiceRepo struct {
	listResult   *boardapi.ListResult[boardapi.InvoiceEntity]
	getResult    *boardapi.InvoiceEntity
	err          error
}

func (s *stubInvoiceRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.InvoiceListOptions) (*boardapi.ListResult[boardapi.InvoiceEntity], error) {
	if s.listResult == nil {
		return &boardapi.ListResult[boardapi.InvoiceEntity]{}, s.err
	}
	return s.listResult, s.err
}
func (s *stubInvoiceRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	return s.getResult, s.err
}

type stubOrderRepo struct {
	getResult *boardapi.OrderEntity
	err       error
}

func (s *stubOrderRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.OrderEntity, error) {
	return s.getResult, s.err
}

type stubDeliveryRepo struct {
	getResult *boardapi.DeliveryEntity
	err       error
}

func (s *stubDeliveryRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.getResult, s.err
}

type stubReceiptRepo struct {
	getResult *boardapi.ReceiptEntity
	err       error
}

func (s *stubReceiptRepo) GetByDocumentID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.getResult, s.err
}

// --- Stubs: vendor ---

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
func (s *stubVendorRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.VendorEntity], error) {
	return nil, s.err
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
func (s *stubVendorBranchRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.VendorBranchEntity], error) {
	return nil, s.err
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
func (s *stubVendorContactRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.VendorContactEntity], error) {
	return nil, s.err
}

type stubPurchaseOrderRepo struct {
	listResult *boardapi.ListResult[boardapi.PurchaseOrderEntity]
	getResult  *boardapi.PurchaseOrderEntity
	err        error
}

func (s *stubPurchaseOrderRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.PurchaseOrderListOptions) (*boardapi.ListResult[boardapi.PurchaseOrderEntity], error) {
	if s.listResult == nil {
		return &boardapi.ListResult[boardapi.PurchaseOrderEntity]{}, s.err
	}
	return s.listResult, s.err
}
func (s *stubPurchaseOrderRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	return s.getResult, s.err
}

type stubPaymentRepo struct {
	listResult *boardapi.ListResult[boardapi.PaymentEntity]
	getResult  *boardapi.PaymentEntity
	err        error
}

func (s *stubPaymentRepo) List(_ context.Context, _ repository.ReadOptions, _ boardapi.PaymentListOptions) (*boardapi.ListResult[boardapi.PaymentEntity], error) {
	if s.listResult == nil {
		return &boardapi.ListResult[boardapi.PaymentEntity]{}, s.err
	}
	return s.listResult, s.err
}
func (s *stubPaymentRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	return s.getResult, s.err
}

// --- Stubs: master ---

type stubUserRepo struct {
	listResult     []boardapi.UserEntity
	getResult      *boardapi.UserEntity
	searchResult   []boardapi.UserEntity
	listPageResult *boardapi.PageResult[boardapi.UserEntity]
	err            error
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
func (s *stubUserRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.UserEntity], error) {
	return s.listPageResult, s.err
}

type stubGroupRepo struct {
	listResult     []boardapi.GroupEntity
	getResult      *boardapi.GroupEntity
	searchResult   []boardapi.GroupEntity
	listPageResult *boardapi.PageResult[boardapi.GroupEntity]
	err            error
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
func (s *stubGroupRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.GroupEntity], error) {
	return s.listPageResult, s.err
}

type stubPaymentTermRepo struct {
	listResult     []boardapi.PaymentTermEntity
	getResult      *boardapi.PaymentTermEntity
	searchResult   []boardapi.PaymentTermEntity
	listPageResult *boardapi.PageResult[boardapi.PaymentTermEntity]
	err            error
}

func (s *stubPaymentTermRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.listResult, s.err
}
func (s *stubPaymentTermRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PaymentTermEntity, error) {
	return s.getResult, s.err
}
func (s *stubPaymentTermRepo) Search(_ context.Context, _ boardapi.PaymentTermSearchParams, _ repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.searchResult, s.err
}
func (s *stubPaymentTermRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.PaymentTermEntity], error) {
	return s.listPageResult, s.err
}

type stubProjectTypeRepo struct {
	listResult   []boardapi.ProjectTypeEntity
	getResult    *boardapi.ProjectTypeEntity
	searchResult []boardapi.ProjectTypeEntity
	err          error
}

func (s *stubProjectTypeRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.listResult, s.err
}
func (s *stubProjectTypeRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.ProjectTypeEntity, error) {
	return s.getResult, s.err
}
func (s *stubProjectTypeRepo) Search(_ context.Context, _ boardapi.ProjectTypeSearchParams, _ repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.searchResult, s.err
}
func (s *stubProjectTypeRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.ProjectTypeEntity], error) {
	return nil, s.err
}

type stubPurchaseTypeRepo struct {
	listResult   []boardapi.PurchaseTypeEntity
	getResult    *boardapi.PurchaseTypeEntity
	searchResult []boardapi.PurchaseTypeEntity
	err          error
}

func (s *stubPurchaseTypeRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.listResult, s.err
}
func (s *stubPurchaseTypeRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error) {
	return s.getResult, s.err
}
func (s *stubPurchaseTypeRepo) Search(_ context.Context, _ boardapi.PurchaseTypeSearchParams, _ repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.searchResult, s.err
}
func (s *stubPurchaseTypeRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.PurchaseTypeEntity], error) {
	return nil, s.err
}

type stubAccountingTypeRepo struct {
	listResult     []boardapi.AccountingTypeEntity
	getResult      *boardapi.AccountingTypeEntity
	searchResult   []boardapi.AccountingTypeEntity
	listPageResult *boardapi.PageResult[boardapi.AccountingTypeEntity]
	err            error
}

func (s *stubAccountingTypeRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.listResult, s.err
}
func (s *stubAccountingTypeRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.AccountingTypeEntity, error) {
	return s.getResult, s.err
}
func (s *stubAccountingTypeRepo) Search(_ context.Context, _ boardapi.AccountingTypeSearchParams, _ repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.searchResult, s.err
}
func (s *stubAccountingTypeRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.AccountingTypeEntity], error) {
	return s.listPageResult, s.err
}

type stubDocumentSendChannelRepo struct {
	listResult     []boardapi.DocumentSendChannelEntity
	getResult      *boardapi.DocumentSendChannelEntity
	searchResult   []boardapi.DocumentSendChannelEntity
	listPageResult *boardapi.PageResult[boardapi.DocumentSendChannelEntity]
	err            error
}

func (s *stubDocumentSendChannelRepo) List(_ context.Context, _ repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.listResult, s.err
}
func (s *stubDocumentSendChannelRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error) {
	return s.getResult, s.err
}
func (s *stubDocumentSendChannelRepo) Search(_ context.Context, _ boardapi.DocumentSendChannelSearchParams, _ repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.searchResult, s.err
}
func (s *stubDocumentSendChannelRepo) ListPage(_ context.Context, _, _ int) (*boardapi.PageResult[boardapi.DocumentSendChannelEntity], error) {
	return s.listPageResult, s.err
}

// --- Zero-value Repos helper ---

func zeroRepos() svcapi.Repos {
	return svcapi.Repos{
		Clients:              &stubClientRepo{},
		ClientBranches:       &stubClientBranchRepo{},
		Contacts:             &stubContactRepo{},
		Projects:             &stubProjectRepo{},
		ProjectCosts:         &stubProjectCostRepo{},
		Estimates:            &stubEstimateRepo{},
		Invoices:             &stubInvoiceRepo{},
		Orders:               &stubOrderRepo{},
		Deliveries:           &stubDeliveryRepo{},
		Receipts:             &stubReceiptRepo{},
		Vendors:              &stubVendorRepo{},
		VendorBranches:       &stubVendorBranchRepo{},
		VendorContacts:       &stubVendorContactRepo{},
		PurchaseOrders:       &stubPurchaseOrderRepo{},
		Payments:             &stubPaymentRepo{},
		Users:                &stubUserRepo{},
		Groups:               &stubGroupRepo{},
		PaymentTerms:         &stubPaymentTermRepo{},
		ProjectTypes:         &stubProjectTypeRepo{},
		PurchaseTypes:        &stubPurchaseTypeRepo{},
		AccountingTypes:      &stubAccountingTypeRepo{},
		DocumentSendChannels: &stubDocumentSendChannelRepo{},
	}
}

// --- Helpers: core ---

func newServiceWithClients(stub *stubClientRepo) *svcapi.Service {
	r := zeroRepos()
	r.Clients = stub
	return svcapi.New(r)
}

func newServiceWithClientBranches(stub *stubClientBranchRepo) *svcapi.Service {
	r := zeroRepos()
	r.ClientBranches = stub
	return svcapi.New(r)
}

func newServiceWithContacts(stub *stubContactRepo) *svcapi.Service {
	r := zeroRepos()
	r.Contacts = stub
	return svcapi.New(r)
}

func newServiceWithProjects(stub *stubProjectRepo) *svcapi.Service {
	r := zeroRepos()
	r.Projects = stub
	return svcapi.New(r)
}

func newServiceWithProjectCosts(stub *stubProjectCostRepo) *svcapi.Service {
	r := zeroRepos()
	r.ProjectCosts = stub
	return svcapi.New(r)
}

// --- Helpers: document ---

func newServiceWithEstimates(stub *stubEstimateRepo) *svcapi.Service {
	r := zeroRepos()
	r.Estimates = stub
	return svcapi.New(r)
}

func newServiceWithInvoices(stub *stubInvoiceRepo) *svcapi.Service {
	r := zeroRepos()
	r.Invoices = stub
	return svcapi.New(r)
}

func newServiceWithOrders(stub *stubOrderRepo) *svcapi.Service {
	r := zeroRepos()
	r.Orders = stub
	return svcapi.New(r)
}

func newServiceWithDeliveries(stub *stubDeliveryRepo) *svcapi.Service {
	r := zeroRepos()
	r.Deliveries = stub
	return svcapi.New(r)
}

func newServiceWithReceipts(stub *stubReceiptRepo) *svcapi.Service {
	r := zeroRepos()
	r.Receipts = stub
	return svcapi.New(r)
}

// --- Helpers: vendor ---

func newServiceWithVendors(stub *stubVendorRepo) *svcapi.Service {
	r := zeroRepos()
	r.Vendors = stub
	return svcapi.New(r)
}

func newServiceWithVendorBranches(stub *stubVendorBranchRepo) *svcapi.Service {
	r := zeroRepos()
	r.VendorBranches = stub
	return svcapi.New(r)
}

func newServiceWithVendorContacts(stub *stubVendorContactRepo) *svcapi.Service {
	r := zeroRepos()
	r.VendorContacts = stub
	return svcapi.New(r)
}

func newServiceWithPurchaseOrders(stub *stubPurchaseOrderRepo) *svcapi.Service {
	r := zeroRepos()
	r.PurchaseOrders = stub
	return svcapi.New(r)
}

func newServiceWithPayments(stub *stubPaymentRepo) *svcapi.Service {
	r := zeroRepos()
	r.Payments = stub
	return svcapi.New(r)
}

// --- Helpers: master ---

func newServiceWithUsers(stub *stubUserRepo) *svcapi.Service {
	r := zeroRepos()
	r.Users = stub
	return svcapi.New(r)
}

func newServiceWithGroups(stub *stubGroupRepo) *svcapi.Service {
	r := zeroRepos()
	r.Groups = stub
	return svcapi.New(r)
}

func newServiceWithPaymentTerms(stub *stubPaymentTermRepo) *svcapi.Service {
	r := zeroRepos()
	r.PaymentTerms = stub
	return svcapi.New(r)
}

func newServiceWithProjectTypes(stub *stubProjectTypeRepo) *svcapi.Service {
	r := zeroRepos()
	r.ProjectTypes = stub
	return svcapi.New(r)
}

func newServiceWithPurchaseTypes(stub *stubPurchaseTypeRepo) *svcapi.Service {
	r := zeroRepos()
	r.PurchaseTypes = stub
	return svcapi.New(r)
}

func newServiceWithAccountingTypes(stub *stubAccountingTypeRepo) *svcapi.Service {
	r := zeroRepos()
	r.AccountingTypes = stub
	return svcapi.New(r)
}

func newServiceWithDocumentSendChannels(stub *stubDocumentSendChannelRepo) *svcapi.Service {
	r := zeroRepos()
	r.DocumentSendChannels = stub
	return svcapi.New(r)
}
