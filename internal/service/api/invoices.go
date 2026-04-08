package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListInvoices は全請求書を返す。
func (s *Service) ListInvoices(ctx context.Context, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.invoices.List(ctx, opts)
}

// GetInvoice は指定 ID の請求書を返す。
func (s *Service) GetInvoice(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	return s.invoices.GetByID(ctx, id, opts)
}

// SearchInvoices はパラメータでフィルタした請求書を返す。
func (s *Service) SearchInvoices(ctx context.Context, params boardapi.InvoiceSearchParams, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.invoices.Search(ctx, params, opts)
}
