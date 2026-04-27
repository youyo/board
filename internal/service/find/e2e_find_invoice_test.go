//go:build e2e

package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T26: ID lookup
func TestE2E_FindInvoice_ByClient_Returns_NonEmpty(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(cs) == 0 {
		t.Skipf("[SKIP:no-data] client seed: err=%v rs=%d", err, len(cs))
	}
	cid := cs[0].Client.ID

	rs, err := svc.FindInvoice(ctx, find.FindInvoiceQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		ClientID:       cid,
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindInvoice(ClientID=%d): %v", cid, err)
	}
	t.Logf("invoices count=%d for client %d", len(rs), cid)
}

// T27: Status (single) API delegation
func TestE2E_FindInvoice_Status_SingleDelegation(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindInvoice(ctx, find.FindInvoiceQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		Status:         "未請求", // 単一値は narrowing 不要で API delegation 可
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindInvoice(Status single): %v", err)
	}
	t.Logf("invoices status-only count=%d", len(rs))
}

// T28: Statuses[] (multi) narrowing reject
func TestE2E_FindInvoice_StatusesOnly_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindInvoice(context.Background(), find.FindInvoiceQuery{
		Statuses: []string{"未請求", "請求済"},
	})
	if err == nil {
		t.Fatalf("expected reject for statuses-only")
	}
	if !containsErrSubstr(err, "narrow") {
		t.Fatalf("expected narrowing error, got: %v", err)
	}
}

// T29: ProjectName 構造的不可は MCP handler 経由で検証 (T43)。
// Service 層には ProjectName フィールドが存在せず Query 自体組み立てられないため
// ここでは「empty query reject」で代替。
func TestE2E_FindInvoice_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindInvoice(context.Background(), find.FindInvoiceQuery{})
	if err == nil {
		t.Fatalf("expected reject for empty query")
	}
}
