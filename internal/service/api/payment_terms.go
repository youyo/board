package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPaymentTerms returns all payment terms.
func (s *Service) ListPaymentTerms(ctx context.Context, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.List(ctx, opts)
}

// GetPaymentTerm returns a payment term by ID.
func (s *Service) GetPaymentTerm(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.GetByID(ctx, id, opts)
}

// SearchPaymentTerms returns payment terms filtered by the given parameters.
func (s *Service) SearchPaymentTerms(ctx context.Context, params boardapi.PaymentTermSearchParams, opts repository.ReadOptions) ([]boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.Search(ctx, params, opts)
}
