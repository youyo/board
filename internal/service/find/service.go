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

// Repos holds all repository dependencies for the find service.
type Repos struct {
	Clients        ClientRepo
	ClientBranches ClientBranchRepo
	Contacts       ContactRepo
	Projects       ProjectRepo
}

// Service is the high-level find service for cross-resource searches.
type Service struct {
	clients        ClientRepo
	clientBranches ClientBranchRepo
	contacts       ContactRepo
	projects       ProjectRepo
}

// New creates a new find Service.
func New(repos Repos) *Service {
	return &Service{
		clients:        repos.Clients,
		clientBranches: repos.ClientBranches,
		contacts:       repos.Contacts,
		projects:       repos.Projects,
	}
}

// repoOpts returns ReadOptions suitable for passing to repositories.
// Limit is stripped because the find layer applies its own limit on aggregated results.
func repoOpts(opts repository.ReadOptions) repository.ReadOptions {
	return repository.ReadOptions{
		Refresh:      opts.Refresh,
		ForceRefresh: opts.ForceRefresh,
	}
}
