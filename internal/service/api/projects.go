package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjects は全案件を返す。
func (s *Service) ListProjects(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.projects.List(ctx, opts)
}

// GetProject は指定 ID の案件を返す。
func (s *Service) GetProject(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	return s.projects.GetByID(ctx, id, opts)
}

// SearchProjects はパラメータでフィルタした案件を返す。
func (s *Service) SearchProjects(ctx context.Context, params boardapi.ProjectSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.projects.Search(ctx, params, opts)
}
