package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// GetReceipt returns a receipt by document ID.
// The returned *ItemResult carries the entity together with response metadata
// (ETag, rate limits) derived from API response headers when the API is called
// directly. Cache hits produce an ItemResult with zero-value ItemMeta.
func (s *Service) GetReceipt(ctx context.Context, documentID int, opts repository.ReadOptions) (*boardapi.ItemResult[boardapi.ReceiptEntity], error) {
	entity, err := s.receipts.GetByDocumentID(ctx, documentID, opts)
	if err != nil {
		return nil, err
	}
	return &boardapi.ItemResult[boardapi.ReceiptEntity]{Item: entity}, nil
}
