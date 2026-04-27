//go:build e2e

// E2E for FindPurchaseOrder (T33-T36).
//
// SKIP-by-design 注意: BOARD アカウントで purchase orders / vendors が 0 件だと
// 全ケースが [SKIP:no-data] になります（tmp/e2e-artifacts/purchase_orders_0.json も 4 byte = null）。
// 仕入運用のあるアカウントで実機検証してください。
package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T34: VendorName resolver (Service には VendorID 渡しなので resolver は MCP 経由 T46 で検証)
func TestE2E_FindPurchaseOrder_ByVendorID(t *testing.T) {
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

	rs, err := svc.FindPurchaseOrder(ctx, find.FindPurchaseOrderQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		VendorID:       vid,
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindPurchaseOrder(VendorID=%d): %v", vid, err)
	}
	t.Logf("purchase orders for vendor %d: count=%d", vid, len(rs))
}

// T35: Statuses[] reject
func TestE2E_FindPurchaseOrder_StatusesOnly_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindPurchaseOrder(context.Background(), find.FindPurchaseOrderQuery{
		Statuses: []string{"発注中", "完了"},
	})
	if err == nil {
		t.Fatalf("expected reject")
	}
	if !containsErrSubstr(err, "narrow") {
		t.Fatalf("expected narrowing error, got: %v", err)
	}
}

// T36: ProjectName 構造的不可（MCP T43 で検証）→ ここは empty reject で代替
func TestE2E_FindPurchaseOrder_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindPurchaseOrder(context.Background(), find.FindPurchaseOrderQuery{})
	if err == nil {
		t.Fatalf("expected reject for empty query")
	}
}
