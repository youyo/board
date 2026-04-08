package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListDeliveries は全納品書を返す。
func (s *Service) ListDeliveries(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.deliveries.List(ctx, opts)
}

// GetDelivery は指定 ID の納品書を返す。
func (s *Service) GetDelivery(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	return s.deliveries.GetByID(ctx, id, opts)
}

// SearchDeliveries はパラメータでフィルタした納品書を返す。
func (s *Service) SearchDeliveries(ctx context.Context, params boardapi.DeliverySearchParams, opts repository.ReadOptions) ([]boardapi.DeliveryEntity, error) {
	return s.deliveries.Search(ctx, params, opts)
}
