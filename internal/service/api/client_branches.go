package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClientBranches returns client branches filtered by the given options.
// Zero filter routes through the local cache; non-zero filter bypasses cache.
func (s *Service) ListClientBranches(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ClientBranchListOptions) (*boardapi.ListResult[boardapi.ClientBranchEntity], error) {
	return s.clientBranches.List(ctx, readOpts, filter)
}

// GetClientBranch returns a client branch by ID.
func (s *Service) GetClientBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error) {
	return s.clientBranches.GetByID(ctx, id, opts)
}
