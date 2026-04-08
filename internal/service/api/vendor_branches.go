package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorBranches は全仕入先支社を返す。
func (s *Service) ListVendorBranches(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.List(ctx, opts)
}

// GetVendorBranch は指定 ID の仕入先支社を返す。
func (s *Service) GetVendorBranch(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.GetByID(ctx, id, opts)
}

// SearchVendorBranches はパラメータでフィルタした仕入先支社を返す。
func (s *Service) SearchVendorBranches(ctx context.Context, params boardapi.VendorBranchSearchParams, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	return s.vendorBranches.Search(ctx, params, opts)
}
