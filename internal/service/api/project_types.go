package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectTypes は全案件種別を返す。
func (s *Service) ListProjectTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.List(ctx, opts)
}

// GetProjectType は指定 ID の案件種別を返す。
func (s *Service) GetProjectType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.GetByID(ctx, id, opts)
}

// SearchProjectTypes はパラメータでフィルタした案件種別を返す。
func (s *Service) SearchProjectTypes(ctx context.Context, params boardapi.ProjectTypeSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.Search(ctx, params, opts)
}
