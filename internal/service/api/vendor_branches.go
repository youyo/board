package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorBranches returns vendor branches filtered by the given options.
// Pass boardapi.VendorBranchListOptions{} for an unfiltered list (cache-backed).
// Non-zero filter bypasses the cache (see repository.VendorBranchRepository.List).
func (s *Service) ListVendorBranches(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.VendorBranchListOptions) (*boardapi.ListResult[boardapi.VendorBranchEntity], error) {
	return s.vendorBranches.List(ctx, readOpts, filter)
}

// GetVendorBranch returns a vendor branch by ID.
func (s *Service) GetVendorBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.GetByID(ctx, id, opts)
}
