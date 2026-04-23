package api_test

import (
	"context"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// --- Common test helpers ---

var testCtx = context.Background()
var defaultOpts = repository.ReadOptions{}

// assertNoError asserts that err is nil.
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertLen asserts that the slice has the expected length.
func assertLen[T any](t *testing.T, got []T, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("len = %d, want %d", len(got), want)
	}
}

// assertNotNil asserts that the pointer is not nil.
func assertNotNil[T any](t *testing.T, got *T) {
	t.Helper()
	if got == nil {
		t.Fatal("got nil, want non-nil")
	}
}

// --- Clients tests ---

func TestListClients(t *testing.T) {
	stub := &stubClientRepo{listResult: []boardapi.ClientEntity{{ID: 1, Name: "ClientA"}}}
	svc := newServiceWithClients(stub)
	got, err := svc.ListClients(testCtx, defaultOpts, boardapi.ClientListOptions{})
	assertNoError(t, err)
	if got == nil {
		t.Fatal("got nil ListResult")
	}
	assertLen(t, got.Items, 1)
}

func TestGetClient(t *testing.T) {
	entity := &boardapi.ClientEntity{ID: 1, Name: "ClientA"}
	stub := &stubClientRepo{getResult: entity}
	svc := newServiceWithClients(stub)
	got, err := svc.GetClient(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// M50: 非ゼロ filter 時は searchResult を返す（cache bypass path の stub 挙動）。
func TestListClients_WithFilter(t *testing.T) {
	stub := &stubClientRepo{
		listResult:   []boardapi.ClientEntity{{ID: 1, Name: "ClientA"}},
		searchResult: []boardapi.ClientEntity{{ID: 2, Name: "ClientB"}},
	}
	svc := newServiceWithClients(stub)
	got, err := svc.ListClients(testCtx, defaultOpts, boardapi.ClientListOptions{NameCont: "ClientB"})
	assertNoError(t, err)
	if got == nil {
		t.Fatal("got nil ListResult")
	}
	assertLen(t, got.Items, 1)
	if got.Items[0].ID != 2 {
		t.Errorf("want ID=2 (from searchResult), got %d", got.Items[0].ID)
	}
}

// --- ClientBranches tests ---

func TestListClientBranches(t *testing.T) {
	stub := &stubClientBranchRepo{listResult: []boardapi.ClientBranchEntity{{ID: 1, Name: "BranchA"}}}
	svc := newServiceWithClientBranches(stub)
	got, err := svc.ListClientBranches(testCtx, defaultOpts, boardapi.ClientBranchListOptions{})
	assertNoError(t, err)
	assertLen(t, got.Items, 1)
}

func TestGetClientBranch(t *testing.T) {
	entity := &boardapi.ClientBranchEntity{ID: 1, Name: "BranchA"}
	stub := &stubClientBranchRepo{getResult: entity}
	svc := newServiceWithClientBranches(stub)
	got, err := svc.GetClientBranch(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Contacts tests ---

func TestListContacts(t *testing.T) {
	stub := &stubContactRepo{listResult: []boardapi.ContactEntity{{ID: 1, LastName: "Contact", FirstName: "A"}}}
	svc := newServiceWithContacts(stub)
	got, err := svc.ListContacts(testCtx, defaultOpts, boardapi.ContactListOptions{})
	assertNoError(t, err)
	assertLen(t, got.Items, 1)
}

func TestGetContact(t *testing.T) {
	entity := &boardapi.ContactEntity{ID: 1, LastName: "Contact", FirstName: "A"}
	stub := &stubContactRepo{getResult: entity}
	svc := newServiceWithContacts(stub)
	got, err := svc.GetContact(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Projects tests ---

func TestListProjects(t *testing.T) {
	stub := &stubProjectRepo{listResult: []boardapi.ProjectEntity{{ID: 1, Name: "ProjectA"}}}
	svc := newServiceWithProjects(stub)
	got, err := svc.ListProjects(testCtx, defaultOpts, boardapi.ProjectListOptions{})
	assertNoError(t, err)
	assertLen(t, got.Items, 1)
}

func TestGetProject(t *testing.T) {
	entity := &boardapi.ProjectEntity{ID: 1, Name: "ProjectA"}
	stub := &stubProjectRepo{getResult: entity}
	svc := newServiceWithProjects(stub)
	got, err := svc.GetProject(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestGetProjectWithGroup(t *testing.T) {
	entity := &boardapi.ProjectEntity{ID: 1, Name: "ProjectA"}
	stub := &stubProjectRepo{getWithGroupResult: entity}
	svc := newServiceWithProjects(stub)
	got, err := svc.GetProjectWithGroup(testCtx, 1, "invoice")
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- ProjectCosts tests ---

func TestListProjectCosts(t *testing.T) {
	stub := &stubProjectCostRepo{listResult: []boardapi.ProjectCostEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithProjectCosts(stub)
	got, err := svc.ListProjectCosts(testCtx, defaultOpts, boardapi.ProjectCostListOptions{})
	assertNoError(t, err)
	assertLen(t, got.Items, 1)
}

func TestGetProjectCost(t *testing.T) {
	entity := &boardapi.ProjectCostEntity{ID: 1, ProjectID: 10}
	stub := &stubProjectCostRepo{getResult: entity}
	svc := newServiceWithProjectCosts(stub)
	got, err := svc.GetProjectCost(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Estimates tests ---

func TestGetEstimate(t *testing.T) {
	entity := &boardapi.EstimateEntity{ID: 1, Total: "90000.0"}
	stub := &stubEstimateRepo{getResult: entity}
	svc := newServiceWithEstimates(stub)
	got, err := svc.GetEstimate(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Invoices tests ---

func TestListInvoices(t *testing.T) {
	stub := &stubInvoiceRepo{listResult: []boardapi.InvoiceEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithInvoices(stub)
	got, err := svc.ListInvoices(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetInvoice(t *testing.T) {
	entity := &boardapi.InvoiceEntity{ID: 1, ProjectID: 10}
	stub := &stubInvoiceRepo{getResult: entity}
	svc := newServiceWithInvoices(stub)
	got, err := svc.GetInvoice(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchInvoices(t *testing.T) {
	stub := &stubInvoiceRepo{searchResult: []boardapi.InvoiceEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithInvoices(stub)
	got, err := svc.SearchInvoices(testCtx, boardapi.InvoiceSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListInvoicesPage(t *testing.T) {
	stub := &stubInvoiceRepo{listPageResult: &boardapi.PageResult[boardapi.InvoiceEntity]{Items: []boardapi.InvoiceEntity{{ID: 1}}}}
	svc := newServiceWithInvoices(stub)
	got, err := svc.ListInvoicesPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Orders tests ---

func TestGetOrder(t *testing.T) {
	entity := &boardapi.OrderEntity{ID: 1, Total: "90000.0"}
	stub := &stubOrderRepo{getResult: entity}
	svc := newServiceWithOrders(stub)
	got, err := svc.GetOrder(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Deliveries tests ---

func TestGetDelivery(t *testing.T) {
	entity := &boardapi.DeliveryEntity{ID: 1, Total: "90000.0", DeliveryDate: "2026-06-30"}
	stub := &stubDeliveryRepo{getResult: entity}
	svc := newServiceWithDeliveries(stub)
	got, err := svc.GetDelivery(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Receipts tests ---

func TestGetReceipt(t *testing.T) {
	entity := &boardapi.ReceiptEntity{ID: 1, Total: "90000.0", ReceiptDate: "2026-06-30"}
	stub := &stubReceiptRepo{getResult: entity}
	svc := newServiceWithReceipts(stub)
	got, err := svc.GetReceipt(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Vendors tests ---

func TestListVendors(t *testing.T) {
	stub := &stubVendorRepo{listResult: []boardapi.VendorEntity{{ID: 1, Name: "VendorA"}}}
	svc := newServiceWithVendors(stub)
	got, err := svc.ListVendors(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendor(t *testing.T) {
	entity := &boardapi.VendorEntity{ID: 1, Name: "VendorA"}
	stub := &stubVendorRepo{getResult: entity}
	svc := newServiceWithVendors(stub)
	got, err := svc.GetVendor(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendors(t *testing.T) {
	stub := &stubVendorRepo{searchResult: []boardapi.VendorEntity{{ID: 2, Name: "VendorB"}}}
	svc := newServiceWithVendors(stub)
	got, err := svc.SearchVendors(testCtx, boardapi.VendorSearchParams{Name: "VendorB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- VendorBranches tests ---

func TestListVendorBranches(t *testing.T) {
	stub := &stubVendorBranchRepo{listResult: []boardapi.VendorBranchEntity{{ID: 1, Vendor: &boardapi.VendorRef{ID: 5}}}}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.ListVendorBranches(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendorBranch(t *testing.T) {
	entity := &boardapi.VendorBranchEntity{ID: 1, Vendor: &boardapi.VendorRef{ID: 5}}
	stub := &stubVendorBranchRepo{getResult: entity}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.GetVendorBranch(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendorBranches(t *testing.T) {
	stub := &stubVendorBranchRepo{searchResult: []boardapi.VendorBranchEntity{{ID: 2, Vendor: &boardapi.VendorRef{ID: 5}}}}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.SearchVendorBranches(testCtx, boardapi.VendorBranchSearchParams{VendorID: 5}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- VendorContacts tests ---

func TestListVendorContacts(t *testing.T) {
	stub := &stubVendorContactRepo{listResult: []boardapi.VendorContactEntity{{ID: 1, Vendor: &boardapi.VendorRef{ID: 5}}}}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.ListVendorContacts(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendorContact(t *testing.T) {
	entity := &boardapi.VendorContactEntity{ID: 1, Vendor: &boardapi.VendorRef{ID: 5}}
	stub := &stubVendorContactRepo{getResult: entity}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.GetVendorContact(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendorContacts(t *testing.T) {
	stub := &stubVendorContactRepo{searchResult: []boardapi.VendorContactEntity{{ID: 2, Vendor: &boardapi.VendorRef{ID: 5}}}}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.SearchVendorContacts(testCtx, boardapi.VendorContactSearchParams{VendorID: 5}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- PurchaseOrders tests ---

func TestListPurchaseOrders(t *testing.T) {
	stub := &stubPurchaseOrderRepo{listResult: []boardapi.PurchaseOrderEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithPurchaseOrders(stub)
	got, err := svc.ListPurchaseOrders(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPurchaseOrder(t *testing.T) {
	entity := &boardapi.PurchaseOrderEntity{ID: 1, ProjectID: 10}
	stub := &stubPurchaseOrderRepo{getResult: entity}
	svc := newServiceWithPurchaseOrders(stub)
	got, err := svc.GetPurchaseOrder(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPurchaseOrders(t *testing.T) {
	stub := &stubPurchaseOrderRepo{searchResult: []boardapi.PurchaseOrderEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithPurchaseOrders(stub)
	got, err := svc.SearchPurchaseOrders(testCtx, boardapi.PurchaseOrderSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Payments tests ---

func TestListPayments(t *testing.T) {
	stub := &stubPaymentRepo{listResult: []boardapi.PaymentEntity{{ID: 1, VendorID: 5}}}
	svc := newServiceWithPayments(stub)
	got, err := svc.ListPayments(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPayment(t *testing.T) {
	entity := &boardapi.PaymentEntity{ID: 1, VendorID: 5}
	stub := &stubPaymentRepo{getResult: entity}
	svc := newServiceWithPayments(stub)
	got, err := svc.GetPayment(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPayments(t *testing.T) {
	stub := &stubPaymentRepo{searchResult: []boardapi.PaymentEntity{{ID: 2, VendorID: 5}}}
	svc := newServiceWithPayments(stub)
	got, err := svc.SearchPayments(testCtx, boardapi.PaymentSearchParams{VendorID: 5}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Users tests ---

func TestListUsers(t *testing.T) {
	stub := &stubUserRepo{listResult: []boardapi.UserEntity{{ID: 1, Name: "UserA"}}}
	svc := newServiceWithUsers(stub)
	got, err := svc.ListUsers(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetUser(t *testing.T) {
	entity := &boardapi.UserEntity{ID: 1, Name: "UserA"}
	stub := &stubUserRepo{getResult: entity}
	svc := newServiceWithUsers(stub)
	got, err := svc.GetUser(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchUsers(t *testing.T) {
	stub := &stubUserRepo{searchResult: []boardapi.UserEntity{{ID: 2, Name: "UserB"}}}
	svc := newServiceWithUsers(stub)
	got, err := svc.SearchUsers(testCtx, boardapi.UserSearchParams{Name: "UserB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListUsersPage(t *testing.T) {
	stub := &stubUserRepo{listPageResult: &boardapi.PageResult[boardapi.UserEntity]{Items: []boardapi.UserEntity{{ID: 1}}}}
	svc := newServiceWithUsers(stub)
	got, err := svc.ListUsersPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- Groups tests ---

func TestListGroups(t *testing.T) {
	stub := &stubGroupRepo{listResult: []boardapi.GroupEntity{{ID: 1, Name: "GroupA"}}}
	svc := newServiceWithGroups(stub)
	got, err := svc.ListGroups(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetGroup(t *testing.T) {
	entity := &boardapi.GroupEntity{ID: 1, Name: "GroupA"}
	stub := &stubGroupRepo{getResult: entity}
	svc := newServiceWithGroups(stub)
	got, err := svc.GetGroup(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchGroups(t *testing.T) {
	stub := &stubGroupRepo{searchResult: []boardapi.GroupEntity{{ID: 2, Name: "GroupB"}}}
	svc := newServiceWithGroups(stub)
	got, err := svc.SearchGroups(testCtx, boardapi.GroupSearchParams{Name: "GroupB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListGroupsPage(t *testing.T) {
	stub := &stubGroupRepo{listPageResult: &boardapi.PageResult[boardapi.GroupEntity]{Items: []boardapi.GroupEntity{{ID: 1}}}}
	svc := newServiceWithGroups(stub)
	got, err := svc.ListGroupsPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- PaymentTerms tests ---

func TestListPaymentTerms(t *testing.T) {
	stub := &stubPaymentTermRepo{listResult: []boardapi.PaymentTermEntity{{ID: 1, Name: "PaymentTermA"}}}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.ListPaymentTerms(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPaymentTerm(t *testing.T) {
	entity := &boardapi.PaymentTermEntity{ID: 1, Name: "PaymentTermA"}
	stub := &stubPaymentTermRepo{getResult: entity}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.GetPaymentTerm(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPaymentTerms(t *testing.T) {
	stub := &stubPaymentTermRepo{searchResult: []boardapi.PaymentTermEntity{{ID: 2, Name: "PaymentTermB"}}}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.SearchPaymentTerms(testCtx, boardapi.PaymentTermSearchParams{Name: "PaymentTermB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListPaymentTermsPage(t *testing.T) {
	stub := &stubPaymentTermRepo{listPageResult: &boardapi.PageResult[boardapi.PaymentTermEntity]{Items: []boardapi.PaymentTermEntity{{ID: 1}}}}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.ListPaymentTermsPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- ProjectTypes tests ---

func TestListProjectTypes(t *testing.T) {
	stub := &stubProjectTypeRepo{listResult: []boardapi.ProjectTypeEntity{{ID: 1, Name: "ProjectTypeA"}}}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.ListProjectTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetProjectType(t *testing.T) {
	entity := &boardapi.ProjectTypeEntity{ID: 1, Name: "ProjectTypeA"}
	stub := &stubProjectTypeRepo{getResult: entity}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.GetProjectType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchProjectTypes(t *testing.T) {
	stub := &stubProjectTypeRepo{searchResult: []boardapi.ProjectTypeEntity{{ID: 2, Name: "ProjectTypeB"}}}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.SearchProjectTypes(testCtx, boardapi.ProjectTypeSearchParams{Name: "ProjectTypeB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- PurchaseTypes tests ---

func TestListPurchaseTypes(t *testing.T) {
	stub := &stubPurchaseTypeRepo{listResult: []boardapi.PurchaseTypeEntity{{ID: 1, Name: "PurchaseTypeA"}}}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.ListPurchaseTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPurchaseType(t *testing.T) {
	entity := &boardapi.PurchaseTypeEntity{ID: 1, Name: "PurchaseTypeA"}
	stub := &stubPurchaseTypeRepo{getResult: entity}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.GetPurchaseType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPurchaseTypes(t *testing.T) {
	stub := &stubPurchaseTypeRepo{searchResult: []boardapi.PurchaseTypeEntity{{ID: 2, Name: "PurchaseTypeB"}}}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.SearchPurchaseTypes(testCtx, boardapi.PurchaseTypeSearchParams{Name: "PurchaseTypeB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- AccountingTypes tests ---

func TestListAccountingTypes(t *testing.T) {
	stub := &stubAccountingTypeRepo{listResult: []boardapi.AccountingTypeEntity{{ID: 1, Name: "AccountingTypeA"}}}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.ListAccountingTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetAccountingType(t *testing.T) {
	entity := &boardapi.AccountingTypeEntity{ID: 1, Name: "AccountingTypeA"}
	stub := &stubAccountingTypeRepo{getResult: entity}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.GetAccountingType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchAccountingTypes(t *testing.T) {
	stub := &stubAccountingTypeRepo{searchResult: []boardapi.AccountingTypeEntity{{ID: 2, Name: "AccountingTypeB"}}}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.SearchAccountingTypes(testCtx, boardapi.AccountingTypeSearchParams{Name: "AccountingTypeB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListAccountingTypesPage(t *testing.T) {
	stub := &stubAccountingTypeRepo{listPageResult: &boardapi.PageResult[boardapi.AccountingTypeEntity]{Items: []boardapi.AccountingTypeEntity{{ID: 1}}}}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.ListAccountingTypesPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}

// --- DocumentSendChannels tests ---

func TestListDocumentSendChannels(t *testing.T) {
	stub := &stubDocumentSendChannelRepo{listResult: []boardapi.DocumentSendChannelEntity{{ID: 1, Name: "DocumentSendChannelA"}}}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.ListDocumentSendChannels(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetDocumentSendChannel(t *testing.T) {
	entity := &boardapi.DocumentSendChannelEntity{ID: 1, Name: "DocumentSendChannelA"}
	stub := &stubDocumentSendChannelRepo{getResult: entity}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.GetDocumentSendChannel(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchDocumentSendChannels(t *testing.T) {
	stub := &stubDocumentSendChannelRepo{searchResult: []boardapi.DocumentSendChannelEntity{{ID: 2, Name: "DocumentSendChannelB"}}}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.SearchDocumentSendChannels(testCtx, boardapi.DocumentSendChannelSearchParams{Name: "DocumentSendChannelB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestListDocumentSendChannelsPage(t *testing.T) {
	stub := &stubDocumentSendChannelRepo{listPageResult: &boardapi.PageResult[boardapi.DocumentSendChannelEntity]{Items: []boardapi.DocumentSendChannelEntity{{ID: 1}}}}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.ListDocumentSendChannelsPage(testCtx, 1, 30)
	assertNoError(t, err)
	assertNotNil(t, got)
}
