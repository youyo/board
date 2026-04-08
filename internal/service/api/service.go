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

// Service は service/api 層のメインサービス。
// repository の薄いラッパーとして ReadOptions の組み立てと結果の返却のみを担う。
type Service struct {
	clients        ClientRepo
	clientBranches ClientBranchRepo
	contacts       ContactRepo
	projects       ProjectRepo
	projectCosts   ProjectCostRepo
}

// Repos は Service 生成に必要なリポジトリをまとめた構造体。
type Repos struct {
	Clients        ClientRepo
	ClientBranches ClientBranchRepo
	Contacts       ContactRepo
	Projects       ProjectRepo
	ProjectCosts   ProjectCostRepo
}

// New は Service を生成する。
func New(repos Repos) *Service {
	return &Service{
		clients:        repos.Clients,
		clientBranches: repos.ClientBranches,
		contacts:       repos.Contacts,
		projects:       repos.Projects,
		projectCosts:   repos.ProjectCosts,
	}
}
