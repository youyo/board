package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetDelivery returns a delivery by document ID.
// The returned *ItemResult carries the entity together with response metadata
// (ETag, rate limits) derived from API response headers when the API is called
// directly. Cache hits produce an ItemResult with zero-value ItemMeta.
func (s *Service) GetDelivery(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ItemResult[boardapi.DeliveryEntity], error) {
	entity, err := s.deliveries.GetByDocumentID(ctx, documentID, opts)
	if err != nil {
		return nil, err
	}
	return &boardapi.ItemResult[boardapi.DeliveryEntity]{Item: entity}, nil
}
