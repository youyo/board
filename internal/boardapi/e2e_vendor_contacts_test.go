//go:build e2e

// E2E tests for /v1/payee_contacts against the real BOARD API.
//
// Scope (M15, board-compliance roadmap): Phase F (vendor) 2nd milestone.
//
//	Note: the boardapi package names this resource "vendor_contacts" but the
//	real BOARD API path is /v1/payee_contacts. This naming mismatch is an
//	existing implementation decision; M15 validates the real API path and
//	records any schema findings without renaming the Go types.
//
//   - List   (TestE2E_VendorContacts_List)   : 1 req expected
//   - Get    (TestE2E_VendorContacts_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_VendorContacts_Search) : 1 req expected
//
// Total budget: 3–4 req (data 0 items: 3 req; data > 0: 4 req; plan cap 8).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// Phase F note: M14 vendor_branches found 0 items for this account, suggesting
// vendor-side resources may have no data. M15 verifies whether the same
// applies to vendor_contacts. List/Search always run regardless of item count.
// Get is skipped only if items == 0 (data-dependent, not a permission issue).
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking contact name / email /
// phone; only lengths, ids, and numeric fields are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_VendorContacts_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_VendorContacts_List exercises GET /v1/payee_contacts and verifies
// that every JSON key returned by the BOARD API is mapped on
// VendorContactEntity. Aggregate statistics (count) are logged without
// leaking individual PII values (name / email / phone / department).
func TestE2E_VendorContacts_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListVendorContactsRaw(ctx, boardapi.VendorContactListOptions{})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListVendorContactsRaw: %v", err)
	}

	dumpJSON(t, "vendor_contacts", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorContactEntity{}); len(diff) > 0 {
		t.Errorf("vendor_contacts list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// name / email / phone / department to avoid PII leakage into CI output.
	var items []boardapi.VendorContactEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_VendorContacts_List: %d items returned", len(items))
}

// TestE2E_VendorContacts_Get discovers a vendor_contact id via List and fetches
// its detail, applying strict field diff on the single-object response.
// Phase F note: M14 vendor_branches found 0 items (data-dependent skip pattern).
// M15 expects the same for vendor_contacts. If items == 0, the test is skipped
// and tracked under "Pending Re-verification" in the roadmap.
// A 404 or 403 on a found ID is surfaced via t.Fatalf (not skipped).
func TestE2E_VendorContacts_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListVendorContactsRaw(ctx, boardapi.VendorContactListOptions{PerPage: 1})
	if err != nil {
		t.Fatalf("ListVendorContactsRaw (discovery): %v", err)
	}
	var items []boardapi.VendorContactEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("vendor_contacts list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first vendor_contact has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetVendorContactRaw(ctx, id)
	if err != nil {
		// Phase F note: Get 404 would indicate vendor resources do not support
		// individual Get, which would be a new finding. 403 is similarly
		// surfaced. Neither is a skip — both fail the test.
		t.Fatalf("GetVendorContactRaw(%d): %v", id, err)
	}

	dumpJSON(t, "vendor_contacts", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.VendorContactEntity{}); len(diff) > 0 {
		t.Errorf("vendor_contacts get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.VendorContactEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetVendorContact ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	// M42 再設計: VendorID() は accessor（nested Vendor.ID）、Name/NameKana/Phone/Memo/VendorBranchID は廃止。
	// Title/Department/Email/Note は *string のため nil ガード付きで長さを計算する。
	titleLen, deptLen, emailLen, noteLen := 0, 0, 0, 0
	if got.Title != nil {
		titleLen = len(*got.Title)
	}
	if got.Department != nil {
		deptLen = len(*got.Department)
	}
	if got.Email != nil {
		emailLen = len(*got.Email)
	}
	if got.Note != nil {
		noteLen = len(*got.Note)
	}
	t.Logf("TestE2E_VendorContacts_Get: id=%d vendor_id=%d archive_flg=%d last_name_len=%d first_name_len=%d honorific_title_len=%d title_len=%d department_len=%d email_len=%d note_len=%d",
		got.ID, got.VendorID(), got.ArchiveFlg,
		len(got.LastName), len(got.FirstName),
		len(got.HonorificTitle), titleLen, deptLen, emailLen, noteLen)
}

// TestE2E_VendorContacts_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field diff.
// Phase F note: the BOARD API has been observed to ignore the `name` filter
// across 9 consecutive milestones (M03-M13). Search is expected to return
// every vendor_contact regardless of the query value; this test focuses on
// StrictFieldDiff and artifact collection rather than server-side filtering.
func TestE2E_VendorContacts_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListVendorContactsRaw(ctx, boardapi.VendorContactListOptions{
		NameCont: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("ListVendorContactsRaw(NameCont=zzz...): %v", err)
	}

	dumpJSON(t, "vendor_contacts_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.VendorContactEntity{}); len(diff) > 0 {
		t.Errorf("vendor_contacts search unmapped fields: %v", diff)
	}

	var items []boardapi.VendorContactEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_VendorContacts_Search: %d items returned", len(items))
}
