package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClientBranches は全顧客支社を返す。
func (s *Service) ListClientBranches(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.clientBranches.List(ctx, opts)
}

// GetClientBranch は指定 ID の顧客支社を返す。
func (s *Service) GetClientBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientBranchEntity, error) {
	return s.clientBranches.GetByID(ctx, id, opts)
}

// SearchClientBranches はパラメータでフィルタした顧客支社を返す。
func (s *Service) SearchClientBranches(ctx context.Context, params boardapi.ClientBranchSearchParams, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	return s.clientBranches.Search(ctx, params, opts)
}
