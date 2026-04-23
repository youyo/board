package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPurchaseTypes returns purchase types filtered by the given options.
// Pass boardapi.PurchaseTypeListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.PurchaseTypeRepository.List).
func (s *Service) ListPurchaseTypes(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PurchaseTypeListOptions) (*boardapi.ListResult[boardapi.PurchaseTypeEntity], error) {
	return s.purchaseTypes.List(ctx, readOpts, filter)
}

// GetPurchaseType returns a purchase type by ID.
func (s *Service) GetPurchaseType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PurchaseTypeEntity, error) {
	return s.purchaseTypes.GetByID(ctx, id, opts)
}
