package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListGroups returns all groups.
func (s *Service) ListGroups(ctx context.Context, opts repository.ReadOptions) ([]boardapi.GroupEntity, error) {
	return s.groups.List(ctx, opts)
}

// GetGroup returns a group by ID.
func (s *Service) GetGroup(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error) {
	return s.groups.GetByID(ctx, id, opts)
}

// SearchGroups returns groups filtered by the given parameters.
func (s *Service) SearchGroups(ctx context.Context, params boardapi.GroupSearchParams, opts repository.ReadOptions) ([]boardapi.GroupEntity, error) {
	return s.groups.Search(ctx, params, opts)
}

// ListGroupsPage returns a single page of groups.
// TODO(M57): PageResult は M57 で ListResult[T] に移行予定。
//
//nolint:staticcheck
func (s *Service) ListGroupsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.GroupEntity], error) {
	return s.groups.ListPage(ctx, page, perPage)
}
