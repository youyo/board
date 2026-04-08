package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClients returns all clients.
func (s *Service) ListClients(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.clients.List(ctx, opts)
}

// GetClient returns a client by ID.
func (s *Service) GetClient(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error) {
	return s.clients.GetByID(ctx, id, opts)
}

// SearchClients returns clients filtered by the given parameters.
func (s *Service) SearchClients(ctx context.Context, params boardapi.ClientSearchParams, opts repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	return s.clients.Search(ctx, params, opts)
}
