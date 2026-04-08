package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindPayment performs a cross-resource search for payments, returning
// payments with their associated vendor.
// Field priority: ID > VendorName > PurchaseOrderID > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
func (s *Service) FindPayment(ctx context.Context, q FindPaymentQuery) ([]PaymentResult, error) {
	if q.ID == 0 && q.VendorName == "" && q.PurchaseOrderID == 0 && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, VendorName, PurchaseOrderID, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var payments []boardapi.PaymentEntity

	switch {
	case q.ID != 0:
		p, err := s.payments.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		payments = []boardapi.PaymentEntity{*p}

	case q.VendorName != "":
		vendors, err := s.vendors.Search(ctx, boardapi.VendorSearchParams{Name: q.VendorName}, opts)
		if err != nil {
			return nil, err
		}
		for _, v := range vendors {
			ps, err := s.payments.Search(ctx, boardapi.PaymentSearchParams{VendorID: v.ID}, opts)
			if err != nil {
				return nil, err
			}
			payments = append(payments, ps...)
		}

	case q.PurchaseOrderID != 0:
		ps, err := s.payments.Search(ctx, boardapi.PaymentSearchParams{PurchaseOrderID: q.PurchaseOrderID}, opts)
		if err != nil {
			return nil, err
		}
		payments = ps

	case q.Text != "":
		all, err := s.payments.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if containsText(q.Text, p.Memo) {
				payments = append(payments, p)
			}
		}

	case q.Status != "":
		all, err := s.payments.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if p.Status == q.Status {
				payments = append(payments, p)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && !(q.VendorName == "" && q.PurchaseOrderID == 0 && q.Text == "") {
		payments = filterPaymentsByStatus(payments, q.Status)
	}

	// Build results with vendor resolution
	results := make([]PaymentResult, 0, len(payments))
	for _, p := range payments {
		var vendor *boardapi.VendorEntity
		if p.VendorID != 0 {
			v, err := s.vendors.GetByID(ctx, p.VendorID, opts)
			if err == nil {
				vendor = v
			}
		}
		results = append(results, PaymentResult{
			Payment: p,
			Vendor:  vendor,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterPaymentsByStatus filters payments by status.
func filterPaymentsByStatus(payments []boardapi.PaymentEntity, status string) []boardapi.PaymentEntity {
	filtered := make([]boardapi.PaymentEntity, 0, len(payments))
	for _, p := range payments {
		if p.Status == status {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
