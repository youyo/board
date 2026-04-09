package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListDocumentSendChannels returns all document send channels.
func (s *Service) ListDocumentSendChannels(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.List(ctx, opts)
}

// GetDocumentSendChannel returns a document send channel by ID.
func (s *Service) GetDocumentSendChannel(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.GetByID(ctx, id, opts)
}

// SearchDocumentSendChannels returns document send channels filtered by the given parameters.
func (s *Service) SearchDocumentSendChannels(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.Search(ctx, params, opts)
}

// ListDocumentSendChannelsPage returns a single page of document send channels.
func (s *Service) ListDocumentSendChannelsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.DocumentSendChannelEntity], error) {
	return s.documentSendChannels.ListPage(ctx, page, perPage)
}
