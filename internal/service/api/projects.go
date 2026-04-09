package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjects returns all projects.
func (s *Service) ListProjects(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.projects.List(ctx, opts)
}

// GetProject returns a project by ID.
func (s *Service) GetProject(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	return s.projects.GetByID(ctx, id, opts)
}

// SearchProjects returns projects filtered by the given parameters.
func (s *Service) SearchProjects(ctx context.Context, params boardapi.ProjectSearchParams, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	return s.projects.Search(ctx, params, opts)
}

// ListProjectsPage returns a single page of projects.
func (s *Service) ListProjectsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ProjectEntity], error) {
	return s.projects.ListPage(ctx, page, perPage)
}

// GetProjectWithGroup returns a project by ID with a response group.
func (s *Service) GetProjectWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error) {
	return s.projects.GetByIDWithGroup(ctx, id, responseGroup)
}
