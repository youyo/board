package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetEstimate returns an estimate by document ID.
func (s *Service) GetEstimate(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	return s.estimates.GetByDocumentID(ctx, documentID, opts)
}
