package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListReceipts returns all receipts.
func (s *Service) ListReceipts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.receipts.List(ctx, opts)
}

// GetReceipt returns a receipt by ID.
func (s *Service) GetReceipt(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.receipts.GetByID(ctx, id, opts)
}

// SearchReceipts returns receipts filtered by the given parameters.
func (s *Service) SearchReceipts(ctx context.Context, params boardapi.ReceiptSearchParams, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.receipts.Search(ctx, params, opts)
}
