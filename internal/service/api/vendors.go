package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendors returns vendors filtered by the given options.
// Pass boardapi.VendorListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.VendorRepository.List).
func (s *Service) ListVendors(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorListOptions) (*boardapi.ListResult[boardapi.VendorEntity], error) {
	return s.vendors.List(ctx, readOpts, filter)
}

// GetVendor returns a vendor by ID.
func (s *Service) GetVendor(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error) {
	return s.vendors.GetByID(ctx, id, opts)
}
