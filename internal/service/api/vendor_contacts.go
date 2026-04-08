package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorContacts returns all vendor contacts.
func (s *Service) ListVendorContacts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.vendorContacts.List(ctx, opts)
}

// GetVendorContact returns a vendor contact by ID.
func (s *Service) GetVendorContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error) {
	return s.vendorContacts.GetByID(ctx, id, opts)
}

// SearchVendorContacts returns vendor contacts filtered by the given parameters.
func (s *Service) SearchVendorContacts(ctx context.Context, params boardapi.VendorContactSearchParams, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.vendorContacts.Search(ctx, params, opts)
}
