//go:build e2e

// E2E tests for /v1/payees against the real BOARD API.
//
// Scope (M16, board-compliance roadmap): Phase F (vendor) 3rd milestone.
// Phase F completion milestone.
//
//	Note: the boardapi package names this resource "vendors" but the
//	real BOARD API path is /v1/payees. This naming mismatch is an
//	existing implementation decision; M16 validates the real API path and
//	records any schema findings without renaming the Go types.
//
//   - List   (TestE2E_Vendors_List)   : 1 req expected
//   - Get    (TestE2E_Vendors_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_Vendors_Search) : 1 req expected
//
// Total budget: 3–5 req (data 0 items: 3 req; data > 0: 4 req; plan cap 8).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// Phase F note: M14 vendor_branches and M15 vendor_contacts both found 0 items
// for this account. M16 verifies whether the same applies to vendors (parent
// resource). The parent resource may have data even if branch/contact data is
// absent. List/Search always run regardless of item count.
// Get is skipped only if items == 0 (data-dependent, not a permission issue).
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking vendor name / code /
// memo; only lengths, ids, and numeric fields are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Vendors_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Vendors_List exercises GET /v1/payees and verifies
// that every JSON key returned by the BOARD API is mapped on
// VendorEntity. Aggregate statistics (count) are logged without
// leaking individual PII values (name / code / memo).
func TestE2E_Vendors_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListVendorsRaw(ctx, boardapi.VendorListOptions{})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListVendorsRaw: %v", err)
	}

	dumpJSON(t, "vendors", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorEntity{}); len(diff) > 0 {
		t.Errorf("vendors list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// name / code / memo raw values to avoid PII leakage into CI output.
	var items []boardapi.VendorEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_Vendors_List: %d items returned", len(items))
}

// TestE2E_Vendors_Get discovers a vendor id via List and fetches
// its detail, applying strict field diff on the single-object response.
// Phase F note: M14 vendor_branches and M15 vendor_contacts found 0 items.
// M16 checks whether the parent vendors resource has data. If items == 0,
// the test is skipped and tracked under "Pending Re-verification" in the
// roadmap.
// A 404 or 403 on a found ID is surfaced via t.Fatalf (not skipped).
func TestE2E_Vendors_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListVendorsRaw(ctx, boardapi.VendorListOptions{PerPage: 1})
	if err != nil {
		t.Fatalf("ListVendorsRaw (discovery): %v", err)
	}
	var items []boardapi.VendorEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("vendors list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first vendor has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetVendorRaw(ctx, id)
	if err != nil {
		// Phase F note: Get 404 would indicate vendors do not support
		// individual Get, which would be a new finding. 403 is similarly
		// surfaced. Neither is a skip — both fail the test.
		t.Fatalf("GetVendorRaw(%d): %v", id, err)
	}

	dumpJSON(t, "vendors", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.VendorEntity{}); len(diff) > 0 {
		t.Errorf("vendors get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.VendorEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetVendor ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	// Do NOT log name / code / memo raw values.
	t.Logf("TestE2E_Vendors_Get: id=%d name_len=%d code_len=%d memo_len=%d updated_at=%s",
		got.ID, len(got.Name), len(got.Code), len(got.Memo), got.UpdatedAt)
}

// TestE2E_Vendors_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field diff.
// Phase F note: the BOARD API has been observed to ignore the `name` filter
// across 9 consecutive milestones (M03-M13). Search is expected to return
// every vendor regardless of the query value; this test focuses on
// StrictFieldDiff and artifact collection rather than server-side filtering.
func TestE2E_Vendors_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListVendorsRaw(ctx, boardapi.VendorListOptions{
		NameCont: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("ListVendorsRaw(NameCont=zzz...): %v", err)
	}

	dumpJSON(t, "vendors_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorEntity{}); len(diff) > 0 {
		t.Errorf("vendors search unmapped fields: %v", diff)
	}

	var items []boardapi.VendorEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Vendors_Search: %d items returned", len(items))
}
