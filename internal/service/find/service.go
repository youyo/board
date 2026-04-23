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
type ClientBranchRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error)
	Search(ctx context.Context, params boardapi.ClientBranchSearchParams, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
}

// ContactRepo is the repository interface for contacts.
type ContactRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error)
	Search(ctx context.Context, params boardapi.ContactSearchParams, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
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

// InvoiceRepo is the repository interface for invoices.
type InvoiceRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
	Search(ctx context.Context, params boardapi.InvoiceSearchParams, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
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

// VendorRepo is the repository interface for vendors used by service/find.
type VendorRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error)
	Search(ctx context.Context, params boardapi.VendorSearchParams, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
}

// VendorBranchRepo is the repository interface for vendor branches.
type VendorBranchRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error)
	Search(ctx context.Context, params boardapi.VendorBranchSearchParams, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
}

// VendorContactRepo is the repository interface for vendor contacts.
type VendorContactRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error)
	Search(ctx context.Context, params boardapi.VendorContactSearchParams, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
}

// PurchaseOrderRepo is the repository interface for purchase orders.
type PurchaseOrderRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error)
	Search(ctx context.Context, params boardapi.PurchaseOrderSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
}

// PaymentRepo is the repository interface for payments.
type PaymentRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error)
	Search(ctx context.Context, params boardapi.PaymentSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
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
