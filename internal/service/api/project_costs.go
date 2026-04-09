package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectCosts returns all project costs.
func (s *Service) ListProjectCosts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.projectCosts.List(ctx, opts)
}

// GetProjectCost returns a project cost by ID.
func (s *Service) GetProjectCost(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectCostEntity, error) {
	return s.projectCosts.GetByID(ctx, id, opts)
}

// SearchProjectCosts returns project costs filtered by the given parameters.
func (s *Service) SearchProjectCosts(ctx context.Context, params boardapi.ProjectCostSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.projectCosts.Search(ctx, params, opts)
}

// ListProjectCostsPage returns a single page of project costs.
func (s *Service) ListProjectCostsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectCostEntity], error) {
	return s.projectCosts.ListPage(ctx, page, perPage)
}
