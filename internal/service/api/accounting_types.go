package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListAccountingTypes returns accounting types filtered by the given options.
// Pass boardapi.AccountingTypeListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.AccountingTypeRepository.List).
func (s *Service) ListAccountingTypes(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.AccountingTypeListOptions) (*boardapi.ListResult[boardapi.AccountingTypeEntity], error) {
	return s.accountingTypes.List(ctx, readOpts, filter)
}

// GetAccountingType returns an accounting type by ID.
func (s *Service) GetAccountingType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.GetByID(ctx, id, opts)
}
