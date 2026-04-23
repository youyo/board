package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListDocumentSendChannels returns document send channels filtered by the given options.
// Pass boardapi.DocumentSendChannelListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.DocumentSendChannelRepository.List).
func (s *Service) ListDocumentSendChannels(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.DocumentSendChannelListOptions) (*boardapi.ListResult[boardapi.DocumentSendChannelEntity], error) {
	return s.documentSendChannels.List(ctx, readOpts, filter)
}

// GetDocumentSendChannel returns a document send channel by ID.
func (s *Service) GetDocumentSendChannel(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.GetByID(ctx, id, opts)
}
