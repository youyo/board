//go:build e2e

// E2E tests for /v1/invoices against the real BOARD API.
//
// Scope (M22, board-compliance roadmap): Phase G (document) 6th milestone.
// invoices は List/Get/Search の 3 本セット（vendors M16 パターン）。
// このアカウントでは 11,000 件規模の可能性があるため、List/Search はともに
// WithPerPage(1) を指定して API リクエスト数を最小化する。
//
//   - List   (TestE2E_Invoices_List)   : 1 req expected (with WithPerPage(1))
//   - Get    (TestE2E_Invoices_Get)    : 2 req expected (list 1 item to discover id + get)
//   - Search (TestE2E_Invoices_Search) : 1 req expected (with WithPerPage(1), UpdatedAtFrom=2099-01-01)
//
// Total budget: ~4 req (data 0 items: 3 req; data > 0: 4 req; plan cap 20).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking invoice title / memo;
// only lengths, ids, and numeric fields are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Invoices_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Invoices_List exercises GET /v1/invoices and verifies
// that every JSON key returned by the BOARD API is mapped on
// InvoiceEntity. Aggregate statistics (count) are logged without
// leaking individual PII values (title / memo).
// WithPerPage(1) is specified to minimize API requests for accounts
// with large datasets (potentially 11,000+ invoices).
func TestE2E_Invoices_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListInvoicesRaw(ctx, boardapi.InvoiceListOptions{PerPage: 1})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListInvoicesRaw: %v", err)
	}

	dumpJSON(t, "invoices", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.InvoiceEntity{}); len(diff) > 0 {
		t.Errorf("invoices list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// title / memo raw values to avoid PII leakage into CI output.
	var items []boardapi.InvoiceEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_Invoices_List: %d items returned (WithPerPage(1))", len(items))
}

// TestE2E_Invoices_Get discovers an invoice id via List (WithPerPage(1)) and fetches
// its detail, applying strict field diff on the single-object response.
// If items == 0, the test is skipped and tracked under "Pending Re-verification"
// in the roadmap.
// A 404 or 403 on a found ID is surfaced via t.Fatalf (not skipped).
func TestE2E_Invoices_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListInvoicesRaw(ctx, boardapi.InvoiceListOptions{PerPage: 1})
	if err != nil {
		t.Fatalf("ListInvoicesRaw (discovery): %v", err)
	}
	var items []boardapi.InvoiceEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("invoices list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first invoice has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetInvoiceRaw(ctx, id)
	if err != nil {
		// Roadmap rule: 403/429 / 404 must NOT be skipped, they must fail the test.
		t.Fatalf("GetInvoiceRaw(%d): %v", id, err)
	}

	dumpJSON(t, "invoices", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.InvoiceEntity{}); len(diff) > 0 {
		t.Errorf("invoices get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.InvoiceEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetInvoice ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	// Do NOT log title / memo raw values.
	t.Logf("TestE2E_Invoices_Get: id=%d title_len=%d memo_len=%d status=%s updated_at=%s",
		got.ID, len(got.Title), len(got.Memo), got.Status, got.UpdatedAt)
}

// TestE2E_Invoices_FilteredList exercises List with UpdatedAtGteq=2099-01-01 to
// produce an empty result set, verifying that the (empty) JSON array still passes
// strict field diff.
// PerPage=1 is specified to minimize API requests.
// The UpdatedAtGteq far-future filter is intentional: it exercises request encoding
// without triggering full pagination on a large dataset.
func TestE2E_Invoices_FilteredList(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListInvoicesRaw(ctx, boardapi.InvoiceListOptions{
		UpdatedAtGteq: "2099-01-01",
		PerPage:       1,
	})
	if err != nil {
		t.Fatalf("ListInvoicesRaw(UpdatedAtGteq=2099-01-01): %v", err)
	}

	dumpJSON(t, "invoices_filtered", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.InvoiceEntity{}); len(diff) > 0 {
		t.Errorf("invoices filtered list unmapped fields: %v", diff)
	}

	var items []boardapi.InvoiceEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal filtered list: %v", err)
	}
	t.Logf("TestE2E_Invoices_FilteredList: %d items returned (UpdatedAtGteq=2099-01-01)", len(items))
}
