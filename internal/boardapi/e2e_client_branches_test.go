//go:build e2e

// E2E tests for /v1/client_branches against the real BOARD API.
//
// Scope (M09, board-compliance roadmap): Phase D (core business) 1st milestone.
//   - List   (TestE2E_ClientBranches_List)   : 1 req expected
//   - Get    (TestE2E_ClientBranches_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_ClientBranches_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 10, headroom for paginated branches). 1 test 1
// endpoint. No skip on 403/429: the BOARD compliance roadmap requires
// immediate failure so that environment or permission issues are not silently
// masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap
// "Pending Re-verification" table and re-run once data is seeded. Unlike the
// master-table case (accounting_types / groups), client_branches is a
// core-business resource but still optional (many accounts register clients
// without sub-branches), so the zero-item branch is realistic.
//
// Phase D note: M02-M08 established that master-table resources consistently
// return 404 on GET /v1/{resource}/{id} and ignore `name` filters. M09 is the
// first core-business resource where we expect the opposite (GET by id works,
// filters work). Test assertions therefore treat Get 404 / resource-wide 403
// as new findings worth flagging via t.Fatalf (not skipping) — the roadmap
// will capture whichever outcome occurs.
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking branch name / address /
// phone / fax; only lengths and ids are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_ClientBranches_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_ClientBranches_List exercises GET /v1/client_branches and verifies
// that every JSON key returned by the BOARD API is mapped on
// ClientBranchEntity.
func TestE2E_ClientBranches_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListClientBranchesRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListClientBranchesRaw: %v", err)
	}

	dumpJSON(t, "client_branches", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ClientBranchEntity{}); len(diff) > 0 {
		t.Errorf("client_branches list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability. Do NOT log
	// branch name / address / phone / fax to avoid PII leakage into CI output.
	var items []boardapi.ClientBranchEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_ClientBranches_List: %d items returned", len(items))
}

// TestE2E_ClientBranches_Get discovers a client_branch id via List and fetches
// its detail, applying strict field diff on the single-object response.
func TestE2E_ClientBranches_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListClientBranchesRaw(ctx)
	if err != nil {
		t.Fatalf("ListClientBranchesRaw (discovery): %v", err)
	}
	var items []boardapi.ClientBranchEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("client_branches list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first client_branch has non-positive ID: %d", id)
	}

	getRaw, err := client.GetClientBranchRaw(ctx, id)
	if err != nil {
		// Phase D note: Get 404 would mean the 4-consecutive master-table
		// finding extends to core-business. This is a fatal outcome that the
		// roadmap must capture, not a skip. 403 is similarly surfaced.
		t.Fatalf("GetClientBranchRaw(%d): %v", id, err)
	}

	dumpJSON(t, "client_branches", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ClientBranchEntity{}); len(diff) > 0 {
		t.Errorf("client_branches get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.ClientBranchEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetClientBranch ID mismatch: got=%d want=%d", got.ID, id)
	}
	// Log only lengths and the client_id (numeric, not PII) for traceability.
	// M39: フィールドを新スキーマ（zip/pref/address1/address2/tel）に更新。ClientID は accessor 経由。
	telLen := 0
	if got.Tel != nil {
		telLen = len(*got.Tel)
	}
	faxLen := 0
	if got.Fax != nil {
		faxLen = len(*got.Fax)
	}
	t.Logf("TestE2E_ClientBranches_Get: id=%d client_id=%d name_len=%d zip_len=%d pref_len=%d address1_len=%d address2_len=%d tel_len=%d fax_len=%d archive_flg=%d",
		got.ID, got.ClientID(), len(got.Name), len(got.Zip), len(got.Pref), len(got.Address1), len(got.Address2), telLen, faxLen, got.ArchiveFlg)
}

// TestE2E_ClientBranches_Search exercises Search with a non-matching name and
// verifies that the (possibly empty) JSON array still passes strict field
// diff. Phase D note: unlike the 4-consecutive master-table `name`-ignored
// behaviour, core-business resources are expected to honour the filter. If
// the filter is honoured, this search returns 0 items; if ignored, the full
// list is returned. Either way, StrictFieldDiff remains meaningful against
// whatever is returned.
func TestE2E_ClientBranches_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchClientBranchesRaw(ctx, boardapi.ClientBranchSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchClientBranchesRaw: %v", err)
	}

	dumpJSON(t, "client_branches_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ClientBranchEntity{}); len(diff) > 0 {
		t.Errorf("client_branches search unmapped fields: %v", diff)
	}

	var items []boardapi.ClientBranchEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_ClientBranches_Search: %d items returned", len(items))
}
