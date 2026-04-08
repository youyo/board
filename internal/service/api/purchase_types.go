package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseTypes は全購買種別を返す。
func (s *Service) ListPurchaseTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.List(ctx, opts)
}

// GetPurchaseType は指定 ID の購買種別を返す。
func (s *Service) GetPurchaseType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.GetByID(ctx, id, opts)
}

// SearchPurchaseTypes はパラメータでフィルタした購買種別を返す。
func (s *Service) SearchPurchaseTypes(ctx context.Context, params boardapi.PurchaseTypeSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.Search(ctx, params, opts)
}
