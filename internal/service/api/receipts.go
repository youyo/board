package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetReceipt returns a receipt by document ID.
func (s *Service) GetReceipt(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	return s.receipts.GetByDocumentID(ctx, documentID, opts)
}
