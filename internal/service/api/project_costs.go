package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectCosts returns project costs filtered by the given options.
// Zero filter routes through the local cache; non-zero filter bypasses cache.
func (s *Service) ListProjectCosts(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ProjectCostListOptions) (*boardapi.ListResult[boardapi.ProjectCostEntity], error) {
	return s.projectCosts.List(ctx, readOpts, filter)
}

// GetProjectCost returns a project cost by ID.
func (s *Service) GetProjectCost(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectCostEntity, error) {
	return s.projectCosts.GetByID(ctx, id, opts)
}
