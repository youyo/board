package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendors は全仕入先を返す。
func (s *Service) ListVendors(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.vendors.List(ctx, opts)
}

// GetVendor は指定 ID の仕入先を返す。
func (s *Service) GetVendor(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorEntity, error) {
	return s.vendors.GetByID(ctx, id, opts)
}

// SearchVendors はパラメータでフィルタした仕入先を返す。
func (s *Service) SearchVendors(ctx context.Context, params boardapi.VendorSearchParams, opts repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	return s.vendors.Search(ctx, params, opts)
}
