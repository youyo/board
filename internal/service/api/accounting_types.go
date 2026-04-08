package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListAccountingTypes は全勘定科目を返す。
func (s *Service) ListAccountingTypes(ctx context.Context, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.List(ctx, opts)
}

// GetAccountingType は指定 ID の勘定科目を返す。
func (s *Service) GetAccountingType(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.GetByID(ctx, id, opts)
}

// SearchAccountingTypes はパラメータでフィルタした勘定科目を返す。
func (s *Service) SearchAccountingTypes(ctx context.Context, params boardapi.AccountingTypeSearchParams, opts repository.ReadOptions) ([]boardapi.AccountingTypeEntity, error) {
	return s.accountingTypes.Search(ctx, params, opts)
}
