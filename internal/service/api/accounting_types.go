package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListAccountingTypes returns all accounting types.
func (s *Service) ListAccountingTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.List(ctx, opts)
}

// GetAccountingType returns an accounting type by ID.
func (s *Service) GetAccountingType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.GetByID(ctx, id, opts)
}

// SearchAccountingTypes returns accounting types filtered by the given parameters.
func (s *Service) SearchAccountingTypes(ctx context.Context, params boardapi.AccountingTypeSearchParams, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.Search(ctx, params, opts)
}
