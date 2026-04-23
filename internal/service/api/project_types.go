package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListProjectTypes returns project types filtered by the given options.
// Pass boardapi.ProjectTypeListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.ProjectTypeRepository.List).
func (s *Service) ListProjectTypes(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ProjectTypeListOptions) (*boardapi.ListResult[boardapi.ProjectTypeEntity], error) {
	return s.projectTypes.List(ctx, readOpts, filter)
}

// GetProjectType returns a project type by ID.
func (s *Service) GetProjectType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ProjectTypeEntity, error) {
	return s.projectTypes.GetByID(ctx, id, opts)
}
