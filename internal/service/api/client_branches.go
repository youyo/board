package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClientBranches returns all client branches.
func (s *Service) ListClientBranches(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.clientBranches.List(ctx, opts)
}

// GetClientBranch returns a client branch by ID.
func (s *Service) GetClientBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error) {
	return s.clientBranches.GetByID(ctx, id, opts)
}

// SearchClientBranches returns client branches filtered by the given parameters.
func (s *Service) SearchClientBranches(ctx context.Context, params boardapi.ClientBranchSearchParams, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.clientBranches.Search(ctx, params, opts)
}

// ListClientBranchesPage returns a single page of client branches.
func (s *Service) ListClientBranchesPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ClientBranchEntity], error) {
	return s.clientBranches.ListPage(ctx, page, perPage)
}
