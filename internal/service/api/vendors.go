package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendors returns all vendors.
func (s *Service) ListVendors(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.vendors.List(ctx, opts)
}

// GetVendor returns a vendor by ID.
func (s *Service) GetVendor(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error) {
	return s.vendors.GetByID(ctx, id, opts)
}

// SearchVendors returns vendors filtered by the given parameters.
func (s *Service) SearchVendors(ctx context.Context, params boardapi.VendorSearchParams, opts repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.vendors.Search(ctx, params, opts)
}

// ListVendorsPage returns a single page of vendors.
func (s *Service) ListVendorsPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.VendorEntity], error) {
	return s.vendors.ListPage(ctx, page, perPage)
}
