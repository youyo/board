package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListUsers は全ユーザーを返す。
func (s *Service) ListUsers(ctx context.Context, opts repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.users.List(ctx, opts)
}

// GetUser は指定 ID のユーザーを返す。
func (s *Service) GetUser(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.UserEntity, error) {
	return s.users.GetByID(ctx, id, opts)
}

// SearchUsers はパラメータでフィルタしたユーザーを返す。
func (s *Service) SearchUsers(ctx context.Context, params boardapi.UserSearchParams, opts repository.ReadOptions) ([]boardapi.UserEntity, error) {
	return s.users.Search(ctx, params, opts)
}
