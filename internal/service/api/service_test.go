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

// --- Estimates テスト ---

func TestListEstimates(t *testing.T) {
	stub := &stubEstimateRepo{listResult: []boardapi.EstimateEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithEstimates(stub)
	got, err := svc.ListEstimates(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetEstimate(t *testing.T) {
	entity := &boardapi.EstimateEntity{ID: 1, ProjectID: 10}
	stub := &stubEstimateRepo{getResult: entity}
	svc := newServiceWithEstimates(stub)
	got, err := svc.GetEstimate(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchEstimates(t *testing.T) {
	stub := &stubEstimateRepo{searchResult: []boardapi.EstimateEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithEstimates(stub)
	got, err := svc.SearchEstimates(testCtx, boardapi.EstimateSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Invoices テスト ---

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

// --- Orders テスト ---

func TestListOrders(t *testing.T) {
	stub := &stubOrderRepo{listResult: []boardapi.OrderEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithOrders(stub)
	got, err := svc.ListOrders(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetOrder(t *testing.T) {
	entity := &boardapi.OrderEntity{ID: 1, ProjectID: 10}
	stub := &stubOrderRepo{getResult: entity}
	svc := newServiceWithOrders(stub)
	got, err := svc.GetOrder(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchOrders(t *testing.T) {
	stub := &stubOrderRepo{searchResult: []boardapi.OrderEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithOrders(stub)
	got, err := svc.SearchOrders(testCtx, boardapi.OrderSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Deliveries テスト ---

func TestListDeliveries(t *testing.T) {
	stub := &stubDeliveryRepo{listResult: []boardapi.DeliveryEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithDeliveries(stub)
	got, err := svc.ListDeliveries(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetDelivery(t *testing.T) {
	entity := &boardapi.DeliveryEntity{ID: 1, ProjectID: 10}
	stub := &stubDeliveryRepo{getResult: entity}
	svc := newServiceWithDeliveries(stub)
	got, err := svc.GetDelivery(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchDeliveries(t *testing.T) {
	stub := &stubDeliveryRepo{searchResult: []boardapi.DeliveryEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithDeliveries(stub)
	got, err := svc.SearchDeliveries(testCtx, boardapi.DeliverySearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Receipts テスト ---

func TestListReceipts(t *testing.T) {
	stub := &stubReceiptRepo{listResult: []boardapi.ReceiptEntity{{ID: 1, ProjectID: 10}}}
	svc := newServiceWithReceipts(stub)
	got, err := svc.ListReceipts(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetReceipt(t *testing.T) {
	entity := &boardapi.ReceiptEntity{ID: 1, ProjectID: 10}
	stub := &stubReceiptRepo{getResult: entity}
	svc := newServiceWithReceipts(stub)
	got, err := svc.GetReceipt(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchReceipts(t *testing.T) {
	stub := &stubReceiptRepo{searchResult: []boardapi.ReceiptEntity{{ID: 2, ProjectID: 10}}}
	svc := newServiceWithReceipts(stub)
	got, err := svc.SearchReceipts(testCtx, boardapi.ReceiptSearchParams{ProjectID: 10}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Vendors テスト ---

func TestListVendors(t *testing.T) {
	stub := &stubVendorRepo{listResult: []boardapi.VendorEntity{{ID: 1, Name: "仕入先A"}}}
	svc := newServiceWithVendors(stub)
	got, err := svc.ListVendors(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendor(t *testing.T) {
	entity := &boardapi.VendorEntity{ID: 1, Name: "仕入先A"}
	stub := &stubVendorRepo{getResult: entity}
	svc := newServiceWithVendors(stub)
	got, err := svc.GetVendor(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendors(t *testing.T) {
	stub := &stubVendorRepo{searchResult: []boardapi.VendorEntity{{ID: 2, Name: "仕入先B"}}}
	svc := newServiceWithVendors(stub)
	got, err := svc.SearchVendors(testCtx, boardapi.VendorSearchParams{Name: "仕入先B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- VendorBranches テスト ---

func TestListVendorBranches(t *testing.T) {
	stub := &stubVendorBranchRepo{listResult: []boardapi.VendorBranchEntity{{ID: 1, VendorID: 5}}}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.ListVendorBranches(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendorBranch(t *testing.T) {
	entity := &boardapi.VendorBranchEntity{ID: 1, VendorID: 5}
	stub := &stubVendorBranchRepo{getResult: entity}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.GetVendorBranch(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendorBranches(t *testing.T) {
	stub := &stubVendorBranchRepo{searchResult: []boardapi.VendorBranchEntity{{ID: 2, VendorID: 5}}}
	svc := newServiceWithVendorBranches(stub)
	got, err := svc.SearchVendorBranches(testCtx, boardapi.VendorBranchSearchParams{VendorID: 5}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- VendorContacts テスト ---

func TestListVendorContacts(t *testing.T) {
	stub := &stubVendorContactRepo{listResult: []boardapi.VendorContactEntity{{ID: 1, VendorID: 5}}}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.ListVendorContacts(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetVendorContact(t *testing.T) {
	entity := &boardapi.VendorContactEntity{ID: 1, VendorID: 5}
	stub := &stubVendorContactRepo{getResult: entity}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.GetVendorContact(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchVendorContacts(t *testing.T) {
	stub := &stubVendorContactRepo{searchResult: []boardapi.VendorContactEntity{{ID: 2, VendorID: 5}}}
	svc := newServiceWithVendorContacts(stub)
	got, err := svc.SearchVendorContacts(testCtx, boardapi.VendorContactSearchParams{VendorID: 5}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- PurchaseOrders テスト ---

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

// --- Payments テスト ---

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

// --- Users テスト ---

func TestListUsers(t *testing.T) {
	stub := &stubUserRepo{listResult: []boardapi.UserEntity{{ID: 1, Name: "ユーザーA"}}}
	svc := newServiceWithUsers(stub)
	got, err := svc.ListUsers(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetUser(t *testing.T) {
	entity := &boardapi.UserEntity{ID: 1, Name: "ユーザーA"}
	stub := &stubUserRepo{getResult: entity}
	svc := newServiceWithUsers(stub)
	got, err := svc.GetUser(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchUsers(t *testing.T) {
	stub := &stubUserRepo{searchResult: []boardapi.UserEntity{{ID: 2, Name: "ユーザーB"}}}
	svc := newServiceWithUsers(stub)
	got, err := svc.SearchUsers(testCtx, boardapi.UserSearchParams{Name: "ユーザーB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- Groups テスト ---

func TestListGroups(t *testing.T) {
	stub := &stubGroupRepo{listResult: []boardapi.GroupEntity{{ID: 1, Name: "グループA"}}}
	svc := newServiceWithGroups(stub)
	got, err := svc.ListGroups(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetGroup(t *testing.T) {
	entity := &boardapi.GroupEntity{ID: 1, Name: "グループA"}
	stub := &stubGroupRepo{getResult: entity}
	svc := newServiceWithGroups(stub)
	got, err := svc.GetGroup(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchGroups(t *testing.T) {
	stub := &stubGroupRepo{searchResult: []boardapi.GroupEntity{{ID: 2, Name: "グループB"}}}
	svc := newServiceWithGroups(stub)
	got, err := svc.SearchGroups(testCtx, boardapi.GroupSearchParams{Name: "グループB"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- PaymentTerms テスト ---

func TestListPaymentTerms(t *testing.T) {
	stub := &stubPaymentTermRepo{listResult: []boardapi.PaymentTermEntity{{ID: 1, Name: "支払条件A"}}}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.ListPaymentTerms(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPaymentTerm(t *testing.T) {
	entity := &boardapi.PaymentTermEntity{ID: 1, Name: "支払条件A"}
	stub := &stubPaymentTermRepo{getResult: entity}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.GetPaymentTerm(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPaymentTerms(t *testing.T) {
	stub := &stubPaymentTermRepo{searchResult: []boardapi.PaymentTermEntity{{ID: 2, Name: "支払条件B"}}}
	svc := newServiceWithPaymentTerms(stub)
	got, err := svc.SearchPaymentTerms(testCtx, boardapi.PaymentTermSearchParams{Name: "支払条件B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- ProjectTypes テスト ---

func TestListProjectTypes(t *testing.T) {
	stub := &stubProjectTypeRepo{listResult: []boardapi.ProjectTypeEntity{{ID: 1, Name: "案件種別A"}}}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.ListProjectTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetProjectType(t *testing.T) {
	entity := &boardapi.ProjectTypeEntity{ID: 1, Name: "案件種別A"}
	stub := &stubProjectTypeRepo{getResult: entity}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.GetProjectType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchProjectTypes(t *testing.T) {
	stub := &stubProjectTypeRepo{searchResult: []boardapi.ProjectTypeEntity{{ID: 2, Name: "案件種別B"}}}
	svc := newServiceWithProjectTypes(stub)
	got, err := svc.SearchProjectTypes(testCtx, boardapi.ProjectTypeSearchParams{Name: "案件種別B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- PurchaseTypes テスト ---

func TestListPurchaseTypes(t *testing.T) {
	stub := &stubPurchaseTypeRepo{listResult: []boardapi.PurchaseTypeEntity{{ID: 1, Name: "購買種別A"}}}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.ListPurchaseTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetPurchaseType(t *testing.T) {
	entity := &boardapi.PurchaseTypeEntity{ID: 1, Name: "購買種別A"}
	stub := &stubPurchaseTypeRepo{getResult: entity}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.GetPurchaseType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchPurchaseTypes(t *testing.T) {
	stub := &stubPurchaseTypeRepo{searchResult: []boardapi.PurchaseTypeEntity{{ID: 2, Name: "購買種別B"}}}
	svc := newServiceWithPurchaseTypes(stub)
	got, err := svc.SearchPurchaseTypes(testCtx, boardapi.PurchaseTypeSearchParams{Name: "購買種別B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- AccountingTypes テスト ---

func TestListAccountingTypes(t *testing.T) {
	stub := &stubAccountingTypeRepo{listResult: []boardapi.AccountingTypeEntity{{ID: 1, Name: "勘定科目A"}}}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.ListAccountingTypes(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetAccountingType(t *testing.T) {
	entity := &boardapi.AccountingTypeEntity{ID: 1, Name: "勘定科目A"}
	stub := &stubAccountingTypeRepo{getResult: entity}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.GetAccountingType(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchAccountingTypes(t *testing.T) {
	stub := &stubAccountingTypeRepo{searchResult: []boardapi.AccountingTypeEntity{{ID: 2, Name: "勘定科目B"}}}
	svc := newServiceWithAccountingTypes(stub)
	got, err := svc.SearchAccountingTypes(testCtx, boardapi.AccountingTypeSearchParams{Name: "勘定科目B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

// --- DocumentSendChannels テスト ---

func TestListDocumentSendChannels(t *testing.T) {
	stub := &stubDocumentSendChannelRepo{listResult: []boardapi.DocumentSendChannelEntity{{ID: 1, Name: "送付方法A"}}}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.ListDocumentSendChannels(testCtx, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}

func TestGetDocumentSendChannel(t *testing.T) {
	entity := &boardapi.DocumentSendChannelEntity{ID: 1, Name: "送付方法A"}
	stub := &stubDocumentSendChannelRepo{getResult: entity}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.GetDocumentSendChannel(testCtx, 1, defaultOpts)
	assertNoError(t, err)
	assertNotNil(t, got)
}

func TestSearchDocumentSendChannels(t *testing.T) {
	stub := &stubDocumentSendChannelRepo{searchResult: []boardapi.DocumentSendChannelEntity{{ID: 2, Name: "送付方法B"}}}
	svc := newServiceWithDocumentSendChannels(stub)
	got, err := svc.SearchDocumentSendChannels(testCtx, boardapi.DocumentSendChannelSearchParams{Name: "送付方法B"}, defaultOpts)
	assertNoError(t, err)
	assertLen(t, got, 1)
}
