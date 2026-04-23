package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjects returns projects filtered by the given options.
// Uses the local cache when filter is zero; bypasses cache when filter is non-zero.
func (s *Service) ListProjects(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ProjectListOptions) (*boardapi.ListResult[boardapi.ProjectEntity], error) {
	return s.projects.List(ctx, readOpts, filter)
}

// GetProject returns a project by ID.
func (s *Service) GetProject(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	return s.projects.GetByID(ctx, id, opts)
}

// GetProjectWithGroup returns a project by ID with a response group.
func (s *Service) GetProjectWithGroup(ctx context.Context, id int, responseGroup string) (*boardapi.ProjectEntity, error) {
	return s.projects.GetByIDWithGroup(ctx, id, responseGroup)
}
