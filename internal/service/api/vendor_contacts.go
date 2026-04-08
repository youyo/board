package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListVendorContacts は全仕入先担当者を返す。
func (s *Service) ListVendorContacts(ctx context.Context, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.vendorContacts.List(ctx, opts)
}

// GetVendorContact は指定 ID の仕入先担当者を返す。
func (s *Service) GetVendorContact(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.VendorContactEntity, error) {
	return s.vendorContacts.GetByID(ctx, id, opts)
}

// SearchVendorContacts はパラメータでフィルタした仕入先担当者を返す。
func (s *Service) SearchVendorContacts(ctx context.Context, params boardapi.VendorContactSearchParams, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	return s.vendorContacts.Search(ctx, params, opts)
}
