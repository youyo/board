package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListGroups returns groups filtered by the given options.
// Pass boardapi.GroupListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.GroupRepository.List).
func (s *Service) ListGroups(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.GroupListOptions) (*boardapi.ListResult[boardapi.GroupEntity], error) {
	return s.groups.List(ctx, readOpts, filter)
}

// GetGroup returns a group by ID.
func (s *Service) GetGroup(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error) {
	return s.groups.GetByID(ctx, id, opts)
}
