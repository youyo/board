package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetDelivery returns a delivery by document ID.
func (s *Service) GetDelivery(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.deliveries.GetByDocumentID(ctx, documentID, opts)
}
