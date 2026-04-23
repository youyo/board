package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListInvoices は InvoiceListOptions でフィルタした請求書一覧を返す。
//
// M54 以降: filter を第2引数として受け取り、*ListResult を返す。
// 旧 SearchInvoices / ListInvoicesPage は削除。
func (s *Service) ListInvoices(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.InvoiceListOptions) (*boardapi.ListResult[boardapi.InvoiceEntity], error) {
	return s.invoices.List(ctx, readOpts, filter)
}

// GetInvoice は指定 ID の請求書を返す。
func (s *Service) GetInvoice(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	return s.invoices.GetByID(ctx, id, opts)
}
