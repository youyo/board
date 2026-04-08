package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectCosts は全案件原価を返す。
func (s *Service) ListProjectCosts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.projectCosts.List(ctx, opts)
}

// GetProjectCost は指定 ID の案件原価を返す。
func (s *Service) GetProjectCost(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectCostEntity, error) {
	return s.projectCosts.GetByID(ctx, id, opts)
}

// SearchProjectCosts はパラメータでフィルタした案件原価を返す。
func (s *Service) SearchProjectCosts(ctx context.Context, params boardapi.ProjectCostSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectCostEntity, error) {
	return s.projectCosts.Search(ctx, params, opts)
}
