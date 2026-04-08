package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorBranches returns all vendor branches.
func (s *Service) ListVendorBranches(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.List(ctx, opts)
}

// GetVendorBranch returns a vendor branch by ID.
func (s *Service) GetVendorBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.GetByID(ctx, id, opts)
}

// SearchVendorBranches returns vendor branches filtered by the given parameters.
func (s *Service) SearchVendorBranches(ctx context.Context, params boardapi.VendorBranchSearchParams, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.Search(ctx, params, opts)
}
