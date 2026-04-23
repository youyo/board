package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListUsers returns users filtered by the given options.
// Pass boardapi.UserListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.UserRepository.List).
func (s *Service) ListUsers(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.UserListOptions) (*boardapi.ListResult[boardapi.UserEntity], error) {
	return s.users.List(ctx, readOpts, filter)
}

// GetUser returns a user by ID.
func (s *Service) GetUser(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error) {
	return s.users.GetByID(ctx, id, opts)
}
