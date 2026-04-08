package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPayments returns all payments.
func (s *Service) ListPayments(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.payments.List(ctx, opts)
}

// GetPayment returns a payment by ID.
func (s *Service) GetPayment(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	return s.payments.GetByID(ctx, id, opts)
}

// SearchPayments returns payments filtered by the given parameters.
func (s *Service) SearchPayments(ctx context.Context, params boardapi.PaymentSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	return s.payments.Search(ctx, params, opts)
}
