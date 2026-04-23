package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListClients returns clients filtered by the given options.
// Pass boardapi.ClientListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.ClientRepository.List).
func (s *Service) ListClients(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ClientListOptions) (*boardapi.ListResult[boardapi.ClientEntity], error) {
	return s.clients.List(ctx, readOpts, filter)
}

// GetClient returns a client by ID.
func (s *Service) GetClient(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ClientEntity, error) {
	return s.clients.GetByID(ctx, id, opts)
}
