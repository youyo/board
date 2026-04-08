package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListReceipts は全領収書を返す。
func (s *Service) ListReceipts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.receipts.List(ctx, opts)
}

// GetReceipt は指定 ID の領収書を返す。
func (s *Service) GetReceipt(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.receipts.GetByID(ctx, id, opts)
}

// SearchReceipts はパラメータでフィルタした領収書を返す。
func (s *Service) SearchReceipts(ctx context.Context, params boardapi.ReceiptSearchParams, opts repository.ReadOptions) ([]boardapi.ReceiptEntity, error) {
	return s.receipts.Search(ctx, params, opts)
}
