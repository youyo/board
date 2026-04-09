package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseTypes returns all purchase types.
func (s *Service) ListPurchaseTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.List(ctx, opts)
}

// GetPurchaseType returns a purchase type by ID.
func (s *Service) GetPurchaseType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.GetByID(ctx, id, opts)
}

// SearchPurchaseTypes returns purchase types filtered by the given parameters.
func (s *Service) SearchPurchaseTypes(ctx context.Context, params boardapi.PurchaseTypeSearchParams, opts repository.ReadOptions) ([]boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.Search(ctx, params, opts)
}

// ListPurchaseTypesPage returns a single page of purchase types.
func (s *Service) ListPurchaseTypesPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.PurchaseTypeEntity], error) {
	return s.purchaseTypes.ListPage(ctx, page, perPage)
}
