package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ClientRepo is the repository interface for clients used by service/find.
// Defined independently from service/api (Go interface segregation).
//
// M50: Search now receives ClientListOptions (Ransack-style) instead of the
// legacy ClientSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.ClientListOptions{}, opts) — this avoids a redundant
// List method while keeping the find layer agnostic of *ListResult.
type ClientRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error)
	Search(ctx context.Context, filter boardapi.ClientListOptions, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
}

// ClientBranchRepo is the repository interface for client branches.
//
// M52: Search now receives ClientBranchListOptions (Ransack-style) instead of the
// legacy ClientBranchSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.ClientBranchListOptions{}, opts).
type ClientBranchRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error)
	Search(ctx context.Context, filter boardapi.ClientBranchListOptions, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
}

// ContactRepo is the repository interface for contacts.
//
// M52: Search now receives ContactListOptions (Ransack-style) instead of the
// legacy ContactSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.ContactListOptions{}, opts).
type ContactRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error)
	Search(ctx context.Context, filter boardapi.ContactListOptions, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
}

// ProjectRepo is the repository interface for projects used by service/find.
//
// M51: Search now receives ProjectListOptions (Ransack-style) instead of the
// legacy ProjectSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.ProjectListOptions{}, opts) — this avoids a redundant
// List method while keeping the find layer agnostic of *ListResult.
type ProjectRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error)
	Search(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
	GetByIDWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error)
}

// EstimateRepo is the repository interface for estimates used by service/find.
type EstimateRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error)
}

// InvoiceRepo is the repository interface for invoices used by service/find.
//
// M54: Search now receives InvoiceListOptions (Ransack-style) instead of the
// legacy InvoiceSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.InvoiceListOptions{}, opts).
type InvoiceRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
	Search(ctx context.Context, filter boardapi.InvoiceListOptions, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
}

// OrderRepo is the repository interface for orders used by service/find.
type OrderRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.OrderEntity, error)
}

// DeliveryRepo is the repository interface for deliveries used by service/find.
type DeliveryRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error)
}

// ReceiptRepo is the repository interface for receipts used by service/find.
type ReceiptRepo interface {
	GetByDocumentID(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error)
}

// PurchaseOrderRepo is the repository interface for purchase orders used by service/find.
//
// M54: Search now receives PurchaseOrderListOptions (Ransack-style) instead of the
// legacy PurchaseOrderSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.PurchaseOrderListOptions{}, opts).
type PurchaseOrderRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error)
	Search(ctx context.Context, filter boardapi.PurchaseOrderListOptions, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
}

// PaymentRepo is the repository interface for payments used by service/find.
//
// M54: Search now receives PaymentListOptions (Ransack-style) instead of the
// legacy PaymentSearchParams. "List all entities" is expressed as
// Search(ctx, boardapi.PaymentListOptions{}, opts).
type PaymentRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error)
	Search(ctx context.Context, filter boardapi.PaymentListOptions, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
}

// VendorRepo is the repository interface for vendors used by service/find.
//
// M55 以降: Search は VendorListOptions（Ransack スタイル）を受け取る。
// 旧 VendorSearchParams は削除。List は ListEntities 経由で廃止。
type VendorRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error)
	Search(ctx context.Context, filter boardapi.VendorListOptions, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
	ListEntities(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorListOptions) ([]boardapi.VendorEntity, error)
}

// VendorBranchRepo is the repository interface for vendor branches.
//
// M55 以降: Search は VendorBranchListOptions（Ransack スタイル）を受け取る。
// 旧 VendorBranchSearchParams は削除。
type VendorBranchRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error)
	Search(ctx context.Context, filter boardapi.VendorBranchListOptions, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
}

// VendorContactRepo is the repository interface for vendor contacts.
//
// M55 以降: Search は VendorContactListOptions（Ransack スタイル）を受け取る。
// 旧 VendorContactSearchParams は削除。
type VendorContactRepo interface {
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error)
	Search(ctx context.Context, filter boardapi.VendorContactListOptions, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
}

// UserRepo is the repository interface for users.
type UserRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error)
	Search(ctx context.Context, params boardapi.UserSearchParams, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
}

// GroupRepo is the repository interface for groups.
type GroupRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error)
	Search(ctx context.Context, params boardapi.GroupSearchParams, opts repository.ReadOptions) ([]boardapi.GroupEntity, error)
}

// Repos holds all repository dependencies for the find service.
type Repos struct {
	Clients        ClientRepo
	ClientBranches ClientBranchRepo
	Contacts       ContactRepo
	Projects       ProjectRepo
	Estimates      EstimateRepo
	Invoices       InvoiceRepo
	Orders         OrderRepo
	Deliveries     DeliveryRepo
	Receipts       ReceiptRepo
	Vendors        VendorRepo
	VendorBranches VendorBranchRepo
	VendorContacts VendorContactRepo
	PurchaseOrders PurchaseOrderRepo
	Payments       PaymentRepo
	Users          UserRepo
	Groups         GroupRepo
}

// Service is the high-level find service for cross-resource searches.
type Service struct {
	clients        ClientRepo
	clientBranches ClientBranchRepo
	contacts       ContactRepo
	projects       ProjectRepo
	estimates      EstimateRepo
	invoices       InvoiceRepo
	orders         OrderRepo
	deliveries     DeliveryRepo
	receipts       ReceiptRepo
	vendors        VendorRepo
	vendorBranches VendorBranchRepo
	vendorContacts VendorContactRepo
	purchaseOrders PurchaseOrderRepo
	payments       PaymentRepo
	users          UserRepo
	groups         GroupRepo
}

// New creates a new find Service.
func New(repos Repos) *Service {
	return &Service{
		clients:        repos.Clients,
		clientBranches: repos.ClientBranches,
		contacts:       repos.Contacts,
		projects:       repos.Projects,
		estimates:      repos.Estimates,
		invoices:       repos.Invoices,
		orders:         repos.Orders,
		deliveries:     repos.Deliveries,
		receipts:       repos.Receipts,
		vendors:        repos.Vendors,
		vendorBranches: repos.VendorBranches,
		vendorContacts: repos.VendorContacts,
		purchaseOrders: repos.PurchaseOrders,
		payments:       repos.Payments,
		users:          repos.Users,
		groups:         repos.Groups,
	}
}

// resolveClientAndProject resolves the client and project for a document.
// Both resolutions are non-fatal: nil is returned on lookup error.
func (s *Service) resolveClientAndProject(ctx context.Context, clientID, projectID int, opts repository.ReadOptions) (*boardapi.ClientEntity, *boardapi.ProjectEntity) {
	var client *boardapi.ClientEntity
	var project *boardapi.ProjectEntity
	if clientID != 0 {
		c, err := s.clients.GetByID(ctx, clientID, opts)
		if err == nil {
			client = c
		}
	}
	if projectID != 0 {
		p, err := s.projects.GetByID(ctx, projectID, opts)
		if err == nil {
			project = p
		}
	}
	return client, project
}

// resolveVendorAndProject resolves the vendor and project for a vendor document.
// Both resolutions are non-fatal: nil is returned on lookup error.
func (s *Service) resolveVendorAndProject(ctx context.Context, vendorID, projectID int, opts repository.ReadOptions) (*boardapi.VendorEntity, *boardapi.ProjectEntity) {
	var vendor *boardapi.VendorEntity
	var project *boardapi.ProjectEntity
	if vendorID != 0 {
		v, err := s.vendors.GetByID(ctx, vendorID, opts)
		if err == nil {
			vendor = v
		}
	}
	if projectID != 0 {
		p, err := s.projects.GetByID(ctx, projectID, opts)
		if err == nil {
			project = p
		}
	}
	return vendor, project
}

// repoOpts returns ReadOptions suitable for passing to repositories.
// Limit is stripped because the find layer applies its own limit on aggregated results.
func repoOpts(opts repository.ReadOptions) repository.ReadOptions {
	return repository.ReadOptions{
		Refresh:      opts.Refresh,
		ForceRefresh: opts.ForceRefresh,
	}
}
