package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListInvoices returns all invoices.
func (s *Service) ListInvoices(ctx context.Context, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.invoices.List(ctx, opts)
}

// GetInvoice returns an invoice by ID.
func (s *Service) GetInvoice(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	return s.invoices.GetByID(ctx, id, opts)
}

// SearchInvoices returns invoices filtered by the given parameters.
func (s *Service) SearchInvoices(ctx context.Context, params boardapi.InvoiceSearchParams, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	return s.invoices.Search(ctx, params, opts)
}

// ListInvoicesPage returns a single page of invoices.
func (s *Service) ListInvoicesPage(ctx context.Context, page, perPage int) (*boardapi.PageResult[boardapi.InvoiceEntity], error) {
	return s.invoices.ListPage(ctx, page, perPage)
}
