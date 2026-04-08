package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPaymentTerms は全支払条件を返す。
func (s *Service) ListPaymentTerms(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.List(ctx, opts)
}

// GetPaymentTerm は指定 ID の支払条件を返す。
func (s *Service) GetPaymentTerm(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.GetByID(ctx, id, opts)
}

// SearchPaymentTerms はパラメータでフィルタした支払条件を返す。
func (s *Service) SearchPaymentTerms(ctx context.Context, params boardapi.PaymentTermSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.Search(ctx, params, opts)
}
