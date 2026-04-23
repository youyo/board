package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorContacts returns vendor contacts filtered by the given options.
// Pass boardapi.VendorContactListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.VendorContactRepository.List).
func (s *Service) ListVendorContacts(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorContactListOptions) (*boardapi.ListResult[boardapi.VendorContactEntity], error) {
	return s.vendorContacts.List(ctx, readOpts, filter)
}

// GetVendorContact returns a vendor contact by ID.
func (s *Service) GetVendorContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error) {
	return s.vendorContacts.GetByID(ctx, id, opts)
}
