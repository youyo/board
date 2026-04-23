package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPaymentTerms returns payment terms filtered by the given options.
// Pass boardapi.PaymentTermListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.PaymentTermRepository.List).
func (s *Service) ListPaymentTerms(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PaymentTermListOptions) (*boardapi.ListResult[boardapi.PaymentTermEntity], error) {
	return s.paymentTerms.List(ctx, readOpts, filter)
}

// GetPaymentTerm returns a payment term by ID.
func (s *Service) GetPaymentTerm(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentTermEntity, error) {
	return s.paymentTerms.GetByID(ctx, id, opts)
}
