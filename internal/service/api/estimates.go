package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListEstimates は全見積を返す。
func (s *Service) ListEstimates(ctx context.Context, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.estimates.List(ctx, opts)
}

// GetEstimate は指定 ID の見積を返す。
func (s *Service) GetEstimate(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.estimates.GetByID(ctx, id, opts)
}

// SearchEstimates はパラメータでフィルタした見積を返す。
func (s *Service) SearchEstimates(ctx context.Context, params boardapi.EstimateSearchParams, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.estimates.Search(ctx, params, opts)
}
