package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClients は全顧客を返す。
func (s *Service) ListClients(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.clients.List(ctx, opts)
}

// GetClient は指定 ID の顧客を返す。
func (s *Service) GetClient(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error) {
	return s.clients.GetByID(ctx, id, opts)
}

// SearchClients はパラメータでフィルタした顧客を返す。
func (s *Service) SearchClients(ctx context.Context, params boardapi.ClientSearchParams, opts repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.clients.Search(ctx, params, opts)
}
