package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectTypes returns all project types.
func (s *Service) ListProjectTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.List(ctx, opts)
}

// GetProjectType returns a project type by ID.
func (s *Service) GetProjectType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.GetByID(ctx, id, opts)
}

// SearchProjectTypes returns project types filtered by the given parameters.
func (s *Service) SearchProjectTypes(ctx context.Context, params boardapi.ProjectTypeSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.Search(ctx, params, opts)
}

// ListProjectTypesPage returns a single page of project types.
func (s *Service) ListProjectTypesPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectTypeEntity], error) {
	return s.projectTypes.ListPage(ctx, page, perPage)
}
