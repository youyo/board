package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ClientRepo は clients リポジトリのインターフェース。
type ClientRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error)
	Search(ctx context.Context, params boardapi.ClientSearchParams, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
}

// ClientBranchRepo は client_branches リポジトリのインターフェース。
type ClientBranchRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error)
	Search(ctx context.Context, params boardapi.ClientBranchSearchParams, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
}

// ContactRepo は contacts リポジトリのインターフェース。
type ContactRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error)
	Search(ctx context.Context, params boardapi.ContactSearchParams, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
}

// ProjectRepo は projects リポジトリのインターフェース。
type ProjectRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error)
	Search(ctx context.Context, params boardapi.ProjectSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
}

// ProjectCostRepo は project_costs リポジトリのインターフェース。
type ProjectCostRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectCostEntity, error)
	Search(ctx context.Context, params boardapi.ProjectCostSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error)
}

// EstimateRepo は estimates リポジトリのインターフェース。
type EstimateRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error)
	Search(ctx context.Context, params boardapi.EstimateSearchParams, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error)
}

// InvoiceRepo は invoices リポジトリのインターフェース。
type InvoiceRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
	Search(ctx context.Context, params boardapi.InvoiceSearchParams, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
}

// OrderRepo は orders リポジトリのインターフェース。
type OrderRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.OrderEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.OrderEntity, error)
	Search(ctx context.Context, params boardapi.OrderSearchParams, opts repository.ReadOptions) ([]boardapi.OrderEntity, error)
}

// DeliveryRepo は deliveries リポジトリのインターフェース。
type DeliveryRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error)
	Search(ctx context.Context, params boardapi.DeliverySearchParams, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error)
}

// ReceiptRepo は receipts リポジトリのインターフェース。
type ReceiptRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error)
	Search(ctx context.Context, params boardapi.ReceiptSearchParams, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error)
}

// VendorRepo は vendors リポジトリのインターフェース。
type VendorRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error)
	Search(ctx context.Context, params boardapi.VendorSearchParams, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
}

// VendorBranchRepo は vendor_branches リポジトリのインターフェース。
type VendorBranchRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error)
	Search(ctx context.Context, params boardapi.VendorBranchSearchParams, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
}

// VendorContactRepo は vendor_contacts リポジトリのインターフェース。
type VendorContactRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error)
	Search(ctx context.Context, params boardapi.VendorContactSearchParams, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
}

// PurchaseOrderRepo は purchase_orders リポジトリのインターフェース。
type PurchaseOrderRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error)
	Search(ctx context.Context, params boardapi.PurchaseOrderSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
}

// PaymentRepo は payments リポジトリのインターフェース。
type PaymentRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error)
	Search(ctx context.Context, params boardapi.PaymentSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
}

// UserRepo は users リポジトリのインターフェース。
type UserRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error)
	Search(ctx context.Context, params boardapi.UserSearchParams, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
}

// GroupRepo は groups リポジトリのインターフェース。
type GroupRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error)
	Search(ctx context.Context, params boardapi.GroupSearchParams, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
}

// PaymentTermRepo は payment_terms リポジトリのインターフェース。
type PaymentTermRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentTermEntity, error)
	Search(ctx context.Context, params boardapi.PaymentTermSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error)
}

// ProjectTypeRepo は project_types リポジトリのインターフェース。
type ProjectTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectTypeEntity, error)
	Search(ctx context.Context, params boardapi.ProjectTypeSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error)
}

// PurchaseTypeRepo は purchase_types リポジトリのインターフェース。
type PurchaseTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error)
	Search(ctx context.Context, params boardapi.PurchaseTypeSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error)
}

// AccountingTypeRepo は accounting_types リポジトリのインターフェース。
type AccountingTypeRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.AccountingTypeEntity, error)
	Search(ctx context.Context, params boardapi.AccountingTypeSearchParams, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error)
}

// DocumentSendChannelRepo は document_send_channels リポジトリのインターフェース。
type DocumentSendChannelRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error)
	Search(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error)
}

// Service は service/api 層のメインサービス。
// repository の薄いラッパーとして ReadOptions の組み立てと結果の返却のみを担う。
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

// Repos は Service 生成に必要なリポジトリをまとめた構造体。
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

// New は Service を生成する。
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
