//go:build e2e

// E2E for FindPayment (T37-T40).
//
// SKIP-by-design 注意: BOARD アカウントで payments が 0 件だと全ケースが [SKIP:no-data] です
// (tmp/e2e-artifacts/payments_0.json も 4 byte = null)。Payment.Project = nil 仮説 (D1) は
// データがある環境で T37 (`TestE2E_FindPayment_Search_Smoke`) が assert します。
package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T37: ID/Search lookup
func TestE2E_FindPayment_Search_Smoke(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindPayment(ctx, find.FindPaymentQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		Text:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindPayment: %v", err)
	}
	if len(rs) == 0 {
		t.Skipf("[SKIP:no-data] payments not present (E2E dump 0 件確認済、D1 維持)")
	}

	// Payment.Project = nil 仮説 (D1) の検証
	for i, r := range rs {
		if r.Project != nil {
			t.Errorf("D1 violation at [%d]: Payment.Project should be nil, got=%+v", i, r.Project)
		}
	}
	t.Logf("payments count=%d, all Project==nil verified (D1)", len(rs))
}

// T38: VendorID lookup
func TestE2E_FindPayment_ByVendorID(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vs, err := svc.FindVendor(ctx, find.FindVendorQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(vs) == 0 {
		t.Skipf("[SKIP:no-data] vendor seed: err=%v rs=%d", err, len(vs))
	}
	vid := vs[0].Vendor.ID

	rs, err := svc.FindPayment(ctx, find.FindPaymentQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		VendorID:       vid,
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindPayment(VendorID=%d): %v", vid, err)
	}
	t.Logf("payments for vendor %d: count=%d", vid, len(rs))
}

// T39: Statuses[] reject
func TestE2E_FindPayment_StatusesOnly_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindPayment(context.Background(), find.FindPaymentQuery{
		Statuses: []string{"未払", "支払済"},
	})
	if err == nil {
		t.Fatalf("expected reject")
	}
	if !containsErrSubstr(err, "narrow") {
		t.Fatalf("expected narrowing error, got: %v", err)
	}
}

// T40: PurchaseOrderID 未対応 → Service にフィールド無し → empty reject で代替
func TestE2E_FindPayment_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindPayment(context.Background(), find.FindPaymentQuery{})
	if err == nil {
		t.Fatalf("expected reject")
	}
}
