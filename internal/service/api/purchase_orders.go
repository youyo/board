package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseOrders は PurchaseOrderListOptions でフィルタした発注書一覧を返す。
//
// M54 以降: filter を第2引数として受け取り、*ListResult を返す。
// 旧 SearchPurchaseOrders / ListPurchaseOrdersPage は削除。
func (s *Service) ListPurchaseOrders(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PurchaseOrderListOptions) (*boardapi.ListResult[boardapi.PurchaseOrderEntity], error) {
	return s.purchaseOrders.List(ctx, readOpts, filter)
}

// GetPurchaseOrder は指定 ID の発注書を返す。
func (s *Service) GetPurchaseOrder(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	return s.purchaseOrders.GetByID(ctx, id, opts)
}
