package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListContacts returns all contacts.
func (s *Service) ListContacts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.contacts.List(ctx, opts)
}

// GetContact returns a contact by ID.
func (s *Service) GetContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.ContactEntity, error) {
	return s.contacts.GetByID(ctx, id, opts)
}

// SearchContacts returns contacts filtered by the given parameters.
func (s *Service) SearchContacts(ctx context.Context, params boardapi.ContactSearchParams, opts repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	return s.contacts.Search(ctx, params, opts)
}

// ListContactsPage returns a single page of contacts.
func (s *Service) ListContactsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.ContactEntity], error) {
	return s.contacts.ListPage(ctx, page, perPage)
}
