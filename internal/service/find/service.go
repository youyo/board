package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ClientRepo is the repository interface for clients used by service/find.
// Defined independently from service/api (Go interface segregation).
type ClientRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error)
	Search(ctx context.Context, params boardapi.ClientSearchParams, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
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

// ProjectRepo is the repository interface for projects.
type ProjectRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error)
	Search(ctx context.Context, params boardapi.ProjectSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
}

// EstimateRepo is the repository interface for estimates.
type EstimateRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error)
	Search(ctx context.Context, params boardapi.EstimateSearchParams, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error)
}

// InvoiceRepo is the repository interface for invoices.
type InvoiceRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error)
	Search(ctx context.Context, params boardapi.InvoiceSearchParams, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
}

// OrderRepo is the repository interface for orders.
type OrderRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.OrderEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.OrderEntity, error)
	Search(ctx context.Context, params boardapi.OrderSearchParams, opts repository.ReadOptions) ([]boardapi.OrderEntity, error)
}

// DeliveryRepo is the repository interface for deliveries.
type DeliveryRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error)
	Search(ctx context.Context, params boardapi.DeliverySearchParams, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error)
}

// ReceiptRepo is the repository interface for receipts.
type ReceiptRepo interface {
	List(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error)
	GetByID(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error)
	Search(ctx context.Context, params boardapi.ReceiptSearchParams, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error)
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

// repoOpts returns ReadOptions suitable for passing to repositories.
// Limit is stripped because the find layer applies its own limit on aggregated results.
func repoOpts(opts repository.ReadOptions) repository.ReadOptions {
	return repository.ReadOptions{
		Refresh:      opts.Refresh,
		ForceRefresh: opts.ForceRefresh,
	}
}
