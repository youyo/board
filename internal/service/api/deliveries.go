package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListDeliveries returns all deliveries.
func (s *Service) ListDeliveries(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.deliveries.List(ctx, opts)
}

// GetDelivery returns a delivery by ID.
func (s *Service) GetDelivery(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.deliveries.GetByID(ctx, id, opts)
}

// SearchDeliveries returns deliveries filtered by the given parameters.
func (s *Service) SearchDeliveries(ctx context.Context, params boardapi.DeliverySearchParams, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.deliveries.Search(ctx, params, opts)
}
