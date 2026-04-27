//go:build e2e

// E2E for FindVendor (T30-T32).
//
// SKIP-by-design 注意: 多くの BOARD アカウント環境では vendor が 0 件で運用されているため
// (tmp/e2e-artifacts/vendors_0.json も 4 byte = null)、credentials があっても
// `[SKIP:no-data] vendors not present` で skip するのが通常です。vendor を使う運用環境で
// 実機検証してください。
package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T30: ID lookup
func TestE2E_FindVendor_ByID_Returns_Single(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindVendor(ctx, find.FindVendorQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株", // 任意の vendor を取得
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(rs) == 0 {
		// vendor 0 件の環境（一般的）— SKIP
		t.Skipf("[SKIP:no-data] vendors not present in account: err=%v rs=%d", err, len(rs))
	}
	id := rs[0].Vendor.ID
	got, err := svc.FindVendor(ctx, find.FindVendorQuery{ID: id})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindVendor(ID=%d): %v", id, err)
	}
	if len(got) != 1 || got[0].Vendor.ID != id {
		t.Fatalf("FindVendor(ID=%d): unexpected: %+v", id, got)
	}
}

// T31: NameCont enrichment（branches/contacts）
func TestE2E_FindVendor_ByName_Enriches(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := svc.FindVendor(ctx, find.FindVendorQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(rs) == 0 {
		t.Skipf("[SKIP:no-data] vendors not present: err=%v rs=%d", err, len(rs))
	}
	t.Logf("FindVendor enriched: vendors=%d branches[0]=%d contacts[0]=%d",
		len(rs), len(rs[0].Branches), len(rs[0].Contacts))
}

// T32: 重複候補
func TestE2E_FindVendor_NameCont_Multiple(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := svc.FindVendor(ctx, find.FindVendorQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 10},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindVendor: %v", err)
	}
	if len(rs) < 2 {
		t.Skipf("[SKIP:no-data] need >=2 vendors, got=%d", len(rs))
	}
	t.Logf("vendors multiple: count=%d", len(rs))
}
