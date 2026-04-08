package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPayments は全支払を返す。
func (s *Service) ListPayments(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.payments.List(ctx, opts)
}

// GetPayment は指定 ID の支払を返す。
func (s *Service) GetPayment(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	return s.payments.GetByID(ctx, id, opts)
}

// SearchPayments はパラメータでフィルタした支払を返す。
func (s *Service) SearchPayments(ctx context.Context, params boardapi.PaymentSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.payments.Search(ctx, params, opts)
}
