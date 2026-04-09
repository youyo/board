package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetOrder returns an order by document ID.
func (s *Service) GetOrder(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.OrderEntity, error) {
	return s.orders.GetByDocumentID(ctx, documentID, opts)
}
