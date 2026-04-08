package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListGroups は全グループを返す。
func (s *Service) ListGroups(ctx context.Context, opts repository.ReadOptions) ([]boardapi.GroupEntity, error) {
	return s.groups.List(ctx, opts)
}

// GetGroup は指定 ID のグループを返す。
func (s *Service) GetGroup(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.GroupEntity, error) {
	return s.groups.GetByID(ctx, id, opts)
}

// SearchGroups はパラメータでフィルタしたグループを返す。
func (s *Service) SearchGroups(ctx context.Context, params boardapi.GroupSearchParams, opts repository.ReadOptions) ([]boardapi.GroupEntity, error) {
	return s.groups.Search(ctx, params, opts)
}
