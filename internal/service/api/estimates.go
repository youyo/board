package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetEstimate returns an estimate by document ID.
// The returned *ItemResult carries the entity together with response metadata
// (ETag, rate limits) derived from API response headers when the API is called
// directly. Cache hits produce an ItemResult with zero-value ItemMeta.
func (s *Service) GetEstimate(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ItemResult[boardapi.EstimateEntity], error) {
	entity, err := s.estimates.GetByDocumentID(ctx, documentID, opts)
	if err != nil {
		return nil, err
	}
	return &boardapi.ItemResult[boardapi.EstimateEntity]{Item: entity}, nil
}
