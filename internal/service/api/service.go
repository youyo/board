package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ClientRepo is the interface for the clients repository.
//
// M50 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListClients を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type ClientRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ClientListOptions) (*boardapi.ListResult[boardapi.ClientEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error)
}

// ClientBranchRepo is the interface for the client_branches repository.
//
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListClientBranches を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type ClientBranchRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ClientBranchListOptions) (*boardapi.ListResult[boardapi.ClientBranchEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error)
}

// ContactRepo is the interface for the contacts repository.
//
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListContacts を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type ContactRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ContactListOptions) (*boardapi.ListResult[boardapi.ContactEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error)
}

// ProjectRepo is the interface for the projects repository.
//
// M51 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListProjects を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type ProjectRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ProjectListOptions) (*boardapi.ListResult[boardapi.ProjectEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error)
	GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error)
}

// ProjectCostRepo is the interface for the project_costs repository.
//
// M52 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListProjectCosts を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type ProjectCostRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ProjectCostListOptions) (*boardapi.ListResult[boardapi.ProjectCostEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectCostEntity, error)
}

// EstimateRepo is the interface for the estimates repository.
type EstimateRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error)
}

// InvoiceRepo is the interface for the invoices repository.
//
// M54 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListInvoices を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type InvoiceRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.InvoiceListOptions) (*boardapi.ListResult[boardapi.InvoiceEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
}

// OrderRepo is the interface for the orders repository.
type OrderRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.OrderEntity, error)
}

// DeliveryRepo is the interface for the deliveries repository.
type DeliveryRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error)
}

// ReceiptRepo is the interface for the receipts repository.
type ReceiptRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error)
}

// VendorRepo is the interface for the vendors repository.
//
// M55 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListVendors を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type VendorRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorListOptions) (*boardapi.ListResult[boardapi.VendorEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error)
}

// VendorBranchRepo is the interface for the vendor_branches repository.
//
// M55 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListVendorBranches を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type VendorBranchRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorBranchListOptions) (*boardapi.ListResult[boardapi.VendorBranchEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error)
}

// VendorContactRepo is the interface for the vendor_contacts repository.
//
// M55 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListVendorContacts を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type VendorContactRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorContactListOptions) (*boardapi.ListResult[boardapi.VendorContactEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error)
}

// PurchaseOrderRepo is the interface for the purchase_orders repository.
//
// M54 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListPurchaseOrders を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type PurchaseOrderRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PurchaseOrderListOptions) (*boardapi.ListResult[boardapi.PurchaseOrderEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error)
}

// PaymentRepo is the interface for the payments repository.
//
// M54 以降: List は (readOpts, filter) 二引数化されて *ListResult を返す。
// 非ゼロ filter 時は cache バイパスで api.ListPayments を直接呼ぶ。
// Search / ListPage は削除（破壊的変更）。
type PaymentRepo interface {
	List(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PaymentListOptions) (*boardapi.ListResult[boardapi.PaymentEntity], error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error)
}

// UserRepo is the interface for the users repository.
type UserRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error)
	Search(ctx context.Context, params boardapi.UserSearchParams, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.UserEntity], error) //nolint:staticcheck
}

// GroupRepo is the interface for the groups repository.
type GroupRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error)
	Search(ctx context.Context, params boardapi.GroupSearchParams, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.GroupEntity], error) //nolint:staticcheck
}

// PaymentTermRepo is the interface for the payment_terms repository.
type PaymentTermRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentTermEntity, error)
	Search(ctx context.Context, params boardapi.PaymentTermSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.PaymentTermEntity], error) //nolint:staticcheck
}

// ProjectTypeRepo is the interface for the project_types repository.
type ProjectTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectTypeEntity, error)
	Search(ctx context.Context, params boardapi.ProjectTypeSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectTypeEntity], error) //nolint:staticcheck
}

// PurchaseTypeRepo is the interface for the purchase_types repository.
type PurchaseTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error)
	Search(ctx context.Context, params boardapi.PurchaseTypeSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.PurchaseTypeEntity], error) //nolint:staticcheck
}

// AccountingTypeRepo is the interface for the accounting_types repository.
type AccountingTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.AccountingTypeEntity, error)
	Search(ctx context.Context, params boardapi.AccountingTypeSearchParams, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.AccountingTypeEntity], error) //nolint:staticcheck
}

// DocumentSendChannelRepo is the interface for the document_send_channels repository.
type DocumentSendChannelRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error)
	Search(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error)
	ListPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.DocumentSendChannelEntity], error) //nolint:staticcheck
}

// Service is the main service for the service/api layer.
// It acts as a thin wrapper over repositories, responsible only for building ReadOptions and returning results.
type Service struct {
	clients              ClientRepo
	clientBranches       ClientBranchRepo
	contacts             ContactRepo
	projects             ProjectRepo
	projectCosts         ProjectCostRepo
	estimates            EstimateRepo
	invoices             InvoiceRepo
	orders               OrderRepo
	deliveries           DeliveryRepo
	receipts             ReceiptRepo
	vendors              VendorRepo
	vendorBranches       VendorBranchRepo
	vendorContacts       VendorContactRepo
	purchaseOrders       PurchaseOrderRepo
	payments             PaymentRepo
	users                UserRepo
	groups               GroupRepo
	paymentTerms         PaymentTermRepo
	projectTypes         ProjectTypeRepo
	purchaseTypes        PurchaseTypeRepo
	accountingTypes      AccountingTypeRepo
	documentSendChannels DocumentSendChannelRepo
}

// Repos is a struct that aggregates all repositories required to construct a Service.
type Repos struct {
	Clients              ClientRepo
	ClientBranches       ClientBranchRepo
	Contacts             ContactRepo
	Projects             ProjectRepo
	ProjectCosts         ProjectCostRepo
	Estimates            EstimateRepo
	Invoices             InvoiceRepo
	Orders               OrderRepo
	Deliveries           DeliveryRepo
	Receipts             ReceiptRepo
	Vendors              VendorRepo
	VendorBranches       VendorBranchRepo
	VendorContacts       VendorContactRepo
	PurchaseOrders       PurchaseOrderRepo
	Payments             PaymentRepo
	Users                UserRepo
	Groups               GroupRepo
	PaymentTerms         PaymentTermRepo
	ProjectTypes         ProjectTypeRepo
	PurchaseTypes        PurchaseTypeRepo
	AccountingTypes      AccountingTypeRepo
	DocumentSendChannels DocumentSendChannelRepo
}

// New creates a new Service.
func New(repos Repos) *Service {
	return &Service{
		clients:              repos.Clients,
		clientBranches:       repos.ClientBranches,
		contacts:             repos.Contacts,
		projects:             repos.Projects,
		projectCosts:         repos.ProjectCosts,
		estimates:            repos.Estimates,
		invoices:             repos.Invoices,
		orders:               repos.Orders,
		deliveries:           repos.Deliveries,
		receipts:             repos.Receipts,
		vendors:              repos.Vendors,
		vendorBranches:       repos.VendorBranches,
		vendorContacts:       repos.VendorContacts,
		purchaseOrders:       repos.PurchaseOrders,
		payments:             repos.Payments,
		users:                repos.Users,
		groups:               repos.Groups,
		paymentTerms:         repos.PaymentTerms,
		projectTypes:         repos.ProjectTypes,
		purchaseTypes:        repos.PurchaseTypes,
		accountingTypes:      repos.AccountingTypes,
		documentSendChannels: repos.DocumentSendChannels,
	}
}
