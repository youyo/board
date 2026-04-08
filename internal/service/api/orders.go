package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListOrders は全受注を返す。
func (s *Service) ListOrders(ctx context.Context, opts repository.ReadOptions) ([]boardapi.OrderEntity, error) {
	return s.orders.List(ctx, opts)
}

// GetOrder は指定 ID の受注を返す。
func (s *Service) GetOrder(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.OrderEntity, error) {
	return s.orders.GetByID(ctx, id, opts)
}

// SearchOrders はパラメータでフィルタした受注を返す。
func (s *Service) SearchOrders(ctx context.Context, params boardapi.OrderSearchParams, opts repository.ReadOptions) ([]boardapi.OrderEntity, error) {
	return s.orders.Search(ctx, params, opts)
}
