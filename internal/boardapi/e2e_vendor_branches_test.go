//go:build e2e

// E2E tests for /v1/payee_branches against the real BOARD API.
//
// Scope (M14, board-compliance roadmap): Phase F (vendor) 1st milestone.
//
//	Note: the boardapi package names this resource "vendor_branches" but the
//	real BOARD API path is /v1/payee_branches. This naming mismatch is an
//	existing implementation decision; M14 validates the real API path and
//	records any schema findings without renaming the Go types.
//
//   - List   (TestE2E_VendorBranches_List)   : 1 req expected
//   - Get    (TestE2E_VendorBranches_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_VendorBranches_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 10). 1 endpoint 1 execution. No skip on
// 403/429: the BOARD compliance roadmap requires immediate failure so that
// environment or permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// Phase F note: M09-M13 confirmed core-business resources return 200 on
// Get-by-id and that the `name` filter is ignored across 9 consecutive
// milestones. M14 verifies whether the same patterns hold for vendor
// resources (Phase F first). The response may include a `vendor` (or
// `payee`) nest object analogous to the `client` nest found in
// client_branches (M09). StrictFieldDiff surfaces any such keys as unmapped
// fields, which are recorded for future VendorBranchEntity enhancement.
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking branch name / address /
// phone / fax; only lengths, vendor_id, and id are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_VendorBranches_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_VendorBranches_List exercises GET /v1/payee_branches and verifies
// that every JSON key returned by the BOARD API is mapped on
// VendorBranchEntity. Aggregate statistics (count, vendor_id distribution) are
// logged without leaking individual PII values (name / address / phone / fax).
func TestE2E_VendorBranches_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListVendorBranchesRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListVendorBranchesRaw: %v", err)
	}

	dumpJSON(t, "vendor_branches", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorBranchEntity{}); len(diff) > 0 {
		t.Errorf("vendor_branches list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// branch name / address / phone / fax to avoid PII leakage into CI output.
	var items []boardapi.VendorBranchEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_VendorBranches_List: %d items returned", len(items))
}

// TestE2E_VendorBranches_Get discovers a vendor_branch id via List and fetches
// its detail, applying strict field diff on the single-object response.
// Phase F note: M09-M13 confirmed core-business resources return 200 on
// Get-by-id (5 consecutive). M14 tests whether this pattern holds for vendor
// resources. A 404 or 403 is surfaced via t.Fatalf.
func TestE2E_VendorBranches_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListVendorBranchesRaw(ctx)
	if err != nil {
		t.Fatalf("ListVendorBranchesRaw (discovery): %v", err)
	}
	var items []boardapi.VendorBranchEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("vendor_branches list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first vendor_branch has non-positive ID: %d", id)
	}

	getRaw, err := client.GetVendorBranchRaw(ctx, id)
	if err != nil {
		// Phase F note: Get 404 would indicate vendor resources do not support
		// individual Get, which would be a new finding after 5 consecutive
		// core-business 200s (M09-M13). This is a fatal outcome that the
		// roadmap must capture, not a skip. 403 is similarly surfaced.
		t.Fatalf("GetVendorBranchRaw(%d): %v", id, err)
	}

	dumpJSON(t, "vendor_branches", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.VendorBranchEntity{}); len(diff) > 0 {
		t.Errorf("vendor_branches get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.VendorBranchEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetVendorBranch ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and the vendor_id (numeric, not PII) for traceability.
	// M41 再設計: VendorID() は accessor（nested Vendor.ID）、Address/Phone/Memo/PostalCode は廃止。
	telLen := 0
	if got.Tel != nil {
		telLen = len(*got.Tel)
	}
	faxLen := 0
	if got.Fax != nil {
		faxLen = len(*got.Fax)
	}
	t.Logf("TestE2E_VendorBranches_Get: id=%d vendor_id=%d name_len=%d zip_len=%d pref_len=%d address1_len=%d address2_len=%d tel_len=%d fax_len=%d archive_flg=%d",
		got.ID, got.VendorID(), len(got.Name), len(got.Zip), len(got.Pref), len(got.Address1), len(got.Address2), telLen, faxLen, got.ArchiveFlg)
}

// TestE2E_VendorBranches_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field diff.
// Phase F note: the BOARD API has been observed to ignore the `name` filter
// across 9 consecutive milestones (M03-M13). Search is expected to return
// every vendor_branch regardless of the query value; this test focuses on
// StrictFieldDiff and artifact collection rather than server-side filtering.
func TestE2E_VendorBranches_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchVendorBranchesRaw(ctx, boardapi.VendorBranchSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchVendorBranchesRaw: %v", err)
	}

	dumpJSON(t, "vendor_branches_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorBranchEntity{}); len(diff) > 0 {
		t.Errorf("vendor_branches search unmapped fields: %v", diff)
	}

	var items []boardapi.VendorBranchEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_VendorBranches_Search: %d items returned", len(items))
}
