package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseOrders は全発注書を返す。
func (s *Service) ListPurchaseOrders(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.List(ctx, opts)
}

// GetPurchaseOrder は指定 ID の発注書を返す。
func (s *Service) GetPurchaseOrder(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.GetByID(ctx, id, opts)
}

// SearchPurchaseOrders はパラメータでフィルタした発注書を返す。
func (s *Service) SearchPurchaseOrders(ctx context.Context, params boardapi.PurchaseOrderSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.Search(ctx, params, opts)
}
