package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListEstimates returns all estimates.
func (s *Service) ListEstimates(ctx context.Context, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.estimates.List(ctx, opts)
}

// GetEstimate returns an estimate by ID.
func (s *Service) GetEstimate(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.estimates.GetByID(ctx, id, opts)
}

// SearchEstimates returns estimates filtered by the given parameters.
func (s *Service) SearchEstimates(ctx context.Context, params boardapi.EstimateSearchParams, opts repository.ReadOptions) ([]boardapi.EstimateEntity, error) {
	return s.estimates.Search(ctx, params, opts)
}
