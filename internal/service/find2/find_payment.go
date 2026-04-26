package find2

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
)

// FindPayment は ID / VendorID / Status / Text による支払検索を行う。
// 検索フィールド優先順位: ID > VendorID > Status > Text。
//
// Status / Statuses の扱い:
//   - Status (single) は StatusEq で API delegation 可（full-scan 不要）
//   - VendorID branch / Text branch でも q.Status を Search filter に同梱して narrowing
//   - Statuses (multi) は API 側 StatusIn[] が不在のため post-filter（filterByStatuses）
//   - Statuses-only クエリは validate() で reject 済（N07a D2）
//
// enrichment ポリシー（N07a D1）:
//   - PaymentEntity トップレベルに ProjectID は存在しない（VendorID + PurchaseOrderID のみ）
//   - E2E dump（payments_*.json = null）で実データ 0 件のため 3-hop 検証不可
//   - PaymentResult.Project は常に nil（schema は維持、N09 E2E 再構築時に 3-hop 再検討）
//   - Vendor のみ enrichment（vendors.GetByID 失敗は non-fatal、slog.Warn + Vendor=nil）
//
// Text マッチ対象: Memo のみ（PaymentEntity に Title 無し）。
// ID 検索時は Status post-filter を skip（N05 踏襲、UX 配慮）。
func (s *Service) FindPayment(ctx context.Context, q FindPaymentQuery) ([]PaymentResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var payments []boardapi.PaymentEntity
	switch {
	case q.ID != 0:
		p, err := s.payments.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		payments = []boardapi.PaymentEntity{*p}
	case q.VendorID != 0:
		list, err := s.payments.Search(ctx, boardapi.PaymentListOptions{
			VendorIDEq: q.VendorID,
			StatusEq:   q.Status,
		}, opts)
		if err != nil {
			return nil, err
		}
		payments = list
	case q.Status != "":
		list, err := s.payments.Search(ctx, boardapi.PaymentListOptions{StatusEq: q.Status}, opts)
		if err != nil {
			return nil, err
		}
		payments = list
	case q.Text != "":
		all, err := s.payments.Search(ctx, boardapi.PaymentListOptions{StatusEq: q.Status}, opts)
		if err != nil {
			return nil, err
		}
		for _, x := range all {
			if containsText(q.Text, x.Memo) {
				payments = append(payments, x)
			}
		}
	}

	// post-filter: Statuses (multi) のみ。Status (single) は API delegation 済。
	// ID 検索時は skip（N05 踏襲、UX 配慮）。
	if q.ID == 0 && len(q.Statuses) > 0 {
		payments = filterByStatuses(payments, func(p boardapi.PaymentEntity) string { return p.Status }, q.Statuses)
	}

	results := make([]PaymentResult, 0, len(payments))
	for _, x := range payments {
		var vendor *boardapi.VendorEntity
		if x.VendorID != 0 {
			v, err := s.vendors.GetByID(ctx, x.VendorID, opts)
			if err != nil {
				slog.Warn("find2.FindPayment: vendor enrichment failed",
					"payment_id", x.ID, "vendor_id", x.VendorID, "error", err)
			} else {
				vendor = v
			}
		}
		// D1: Payment.Project は常に nil（PaymentEntity に ProjectID なし、E2E dump 検証不可）
		results = append(results, PaymentResult{Payment: x, Vendor: vendor, Project: nil})
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}
