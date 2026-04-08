package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseOrders returns all purchase orders.
func (s *Service) ListPurchaseOrders(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.List(ctx, opts)
}

// GetPurchaseOrder returns a purchase order by ID.
func (s *Service) GetPurchaseOrder(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.GetByID(ctx, id, opts)
}

// SearchPurchaseOrders returns purchase orders filtered by the given parameters.
func (s *Service) SearchPurchaseOrders(ctx context.Context, params boardapi.PurchaseOrderSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.Search(ctx, params, opts)
}
