package api

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// ListPayments は PaymentListOptions でフィルタした支払一覧を返す。
//
// M54 以降: filter を第2引数として受け取り、*ListResult を返す。
// 旧 SearchPayments / ListPaymentsPage は削除。
func (s *Service) ListPayments(ctx context.Context, readOpts repository.ReadOptions, filter boardapi.PaymentListOptions) (*boardapi.ListResult[boardapi.PaymentEntity], error) {
	return s.payments.List(ctx, readOpts, filter)
}

// GetPayment は指定 ID の支払を返す。
func (s *Service) GetPayment(ctx context.Context, id int, opts repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	return s.payments.GetByID(ctx, id, opts)
}
