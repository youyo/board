package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListDocumentSendChannels は全送付方法を返す。
func (s *Service) ListDocumentSendChannels(ctx context.Context, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.List(ctx, opts)
}

// GetDocumentSendChannel は指定 ID の送付方法を返す。
func (s *Service) GetDocumentSendChannel(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.GetByID(ctx, id, opts)
}

// SearchDocumentSendChannels はパラメータでフィルタした送付方法を返す。
func (s *Service) SearchDocumentSendChannels(ctx context.Context, params boardapi.DocumentSendChannelSearchParams, opts repository.ReadOptions) ([]boardapi.DocumentSendChannelEntity, error) {
	return s.documentSendChannels.Search(ctx, params, opts)
}
