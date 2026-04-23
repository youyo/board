//go:build e2e

// E2E tests for /v1/expenditure_payments against the real BOARD API.
//
// Scope (M24, board-compliance roadmap): Phase G (vendor-side) 8th milestone.
// Phase G 完走。payments は List/Get/Search の 3 本セット（vendors M16 パターン）。
//
//	Note: the boardapi package names this resource "payments" but the
//	real BOARD API path is /v1/expenditure_payments. This naming mismatch is an
//	existing implementation decision; M24 validates the real API path.
//
//   - List   (TestE2E_Payments_List)   : 1 req expected
//   - Get    (TestE2E_Payments_Get)    : 2 req expected (list 1 item to discover id + get)
//   - Search (TestE2E_Payments_Search) : 1 req expected (UpdatedAtFrom=2099-01-01)
//
// Total budget: ~4 req (data 0 items: 3 req; data > 0: 4 req; plan cap 10).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking payment memo;
// only lengths, ids, and numeric fields are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Payments_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Payments_List exercises GET /v1/expenditure_payments and verifies
// that every JSON key returned by the BOARD API is mapped on
// PaymentEntity. Aggregate statistics (count) are logged without
// leaking individual PII values (memo).
func TestE2E_Payments_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListPaymentsRaw(ctx, boardapi.PaymentListOptions{})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListPaymentsRaw: %v", err)
	}

	dumpJSON(t, "payments", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.PaymentEntity{}); len(diff) > 0 {
		t.Errorf("payments list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// memo raw values to avoid PII leakage into CI output.
	var items []boardapi.PaymentEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_Payments_List: %d items returned", len(items))
}

// TestE2E_Payments_Get discovers a payment id via List and fetches
// its detail, applying strict field diff on the single-object response.
// If items == 0, the test is skipped and tracked under "Pending Re-verification"
// in the roadmap.
// A 404 or 403 on a found ID is surfaced via t.Fatalf (not skipped).
func TestE2E_Payments_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListPaymentsRaw(ctx, boardapi.PaymentListOptions{PerPage: 1})
	if err != nil {
		t.Fatalf("ListPaymentsRaw (discovery): %v", err)
	}
	var items []boardapi.PaymentEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("payments list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first payment has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetPaymentRaw(ctx, id)
	if err != nil {
		// Roadmap rule: 403/429 / 404 must NOT be skipped, they must fail the test.
		t.Fatalf("GetPaymentRaw(%d): %v", id, err)
	}

	dumpJSON(t, "payments", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.PaymentEntity{}); len(diff) > 0 {
		t.Errorf("payments get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.PaymentEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetPayment ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	// Do NOT log memo raw values.
	t.Logf("TestE2E_Payments_Get: id=%d memo_len=%d status=%s payment_date=%s updated_at=%s",
		got.ID, len(got.Memo), got.Status, got.PaymentDate, got.UpdatedAt)
}

// TestE2E_Payments_Search exercises Search with UpdatedAtFrom=2099-01-01 to
// produce an empty result set, verifying that the (empty) JSON array still passes
// strict field diff. The far-future filter exercises request encoding without
// triggering full pagination.
func TestE2E_Payments_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListPaymentsRaw(ctx, boardapi.PaymentListOptions{
		UpdatedAtGteq: "2099-01-01",
		PerPage:       1,
	})
	if err != nil {
		t.Fatalf("ListPaymentsRaw(UpdatedAtGteq=2099-01-01): %v", err)
	}

	dumpJSON(t, "payments_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.PaymentEntity{}); len(diff) > 0 {
		t.Errorf("payments search unmapped fields: %v", diff)
	}

	var items []boardapi.PaymentEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Payments_Search: %d items returned (UpdatedAtFrom=2099-01-01)", len(items))
}
