package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListUsers returns all users.
func (s *Service) ListUsers(ctx context.Context, opts repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.users.List(ctx, opts)
}

// GetUser returns a user by ID.
func (s *Service) GetUser(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error) {
	return s.users.GetByID(ctx, id, opts)
}

// SearchUsers returns users filtered by the given parameters.
func (s *Service) SearchUsers(ctx context.Context, params boardapi.UserSearchParams, opts repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.users.Search(ctx, params, opts)
}

// ListUsersPage returns a single page of users.
func (s *Service) ListUsersPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.UserEntity], error) {
	return s.users.ListPage(ctx, page, perPage)
}
