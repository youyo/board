package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListContacts returns contacts filtered by the given options.
// Zero filter routes through the local cache; non-zero filter bypasses cache.
func (s *Service) ListContacts(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.ContactListOptions) (*boardapi.ListResult[boardapi.ContactEntity], error) {
	return s.contacts.List(ctx, readOpts, filter)
}

// GetContact returns a contact by ID.
func (s *Service) GetContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error) {
	return s.contacts.GetByID(ctx, id, opts)
}
