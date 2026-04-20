//go:build e2e

// E2E tests for /v1/clients against the real BOARD API.
//
// Scope (M12, board-compliance roadmap): Phase E (core business re-verification)
// 1st milestone. clients is the top-level parent of branches, contacts,
// projects, invoices and many other child resources.
//   - List   (TestE2E_Clients_List)   : 1 req expected
//   - Get    (TestE2E_Clients_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_Clients_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 5). 1 test 1 endpoint. No skip on 403/429: the
// BOARD compliance roadmap requires immediate failure so that environment or
// permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips with
// "pending re-verification" — tracked in the roadmap "Pending Re-verification"
// table and re-run once data is seeded. For clients zero-item is unrealistic
// on a production account but the skip branch is retained for defensive
// symmetry with M02-M11.
//
// Phase E 1st note: M09 client_branches / M10 contacts / M11 project_costs
// established that core-business resources return 200 on Get-by-id. M12 is
// expected to reproduce the Get 200 outcome. ClientEntity currently exposes
// only 6 fields (ID / Name / Code / Memo / UpdatedAt / CreatedAt) which is
// suspected to be narrower than the actual API response; StrictFieldDiff will
// surface any unmapped keys. The `Memo` key is expected to surface as a
// reverse-direction mismatch (absent from the API) — 7 consecutive milestones
// have confirmed this BOARD-wide pattern. clients being the parent may also
// embed nested child arrays (branches / contacts / payment_term etc.).
//
// PII handling: clients are customer records that typically contain company
// names, customer codes, and internal memos. Raw artifacts are written under
// tmp/e2e-artifacts/ (gitignored) for manual inspection. t.Logf output
// intentionally avoids leaking individual customer names, codes, or memos —
// only lengths, ids, nonzero counts and aggregate stats are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Clients_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Clients_List exercises GET /v1/clients and verifies that every JSON
// key returned by the BOARD API is mapped on ClientEntity. Aggregate
// statistics (name / code / memo non-empty counts) are logged without leaking
// individual values.
func TestE2E_Clients_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListClientsRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListClientsRaw: %v", err)
	}

	dumpJSON(t, "clients", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ClientEntity{}); len(diff) > 0 {
		t.Errorf("clients list unmapped fields: %v", diff)
	}

	var items []boardapi.ClientEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}

	// Aggregate stats only. Never log individual names, codes, or memos.
	var (
		nameFilled int
		codeFilled int
		memoFilled int
	)
	for _, cl := range items {
		if cl.Name != "" {
			nameFilled++
		}
		if cl.Code != "" {
			codeFilled++
		}
		if cl.Memo != "" {
			memoFilled++
		}
	}
	t.Logf("TestE2E_Clients_List: %d items returned", len(items))
	t.Logf("distribution: name_filled=%d/%d code_filled=%d/%d memo_filled=%d/%d",
		nameFilled, len(items),
		codeFilled, len(items),
		memoFilled, len(items),
	)
}

// TestE2E_Clients_Get discovers a client id via List and fetches its detail,
// applying strict field diff on the single-object response. Phase E 1st: 200
// is expected based on M09/M10/M11 precedent; any 404/403 surfaces as a Fatal
// to highlight a regression in core-business Get support.
func TestE2E_Clients_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListClientsRaw(ctx)
	if err != nil {
		t.Fatalf("ListClientsRaw (discovery): %v", err)
	}
	var items []boardapi.ClientEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data means
		// Get cannot be exercised; tracked under "Pending Re-verification" in
		// plans/board-compliance-roadmap.md and re-run once data is seeded.
		t.Skipf("clients list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first client has non-positive ID: %d", id)
	}

	getRaw, err := client.GetClientRaw(ctx, id)
	if err != nil {
		// Phase E note: M09/M10/M11 confirmed core-business resources return
		// 200 on Get-by-id. If M12 regresses to 404 it's a new finding worthy
		// of immediate halt and roadmap capture. 403 is similarly surfaced.
		t.Fatalf("GetClientRaw(%d): %v", id, err)
	}

	dumpJSON(t, "clients", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ClientEntity{}); len(diff) > 0 {
		t.Errorf("clients get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.ClientEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetClient ID mismatch: got=%d want=%d", got.ID, id)
	}

	// Log only lengths and presence booleans. NEVER log the actual name,
	// code, or memo values — client records are commercially sensitive.
	t.Logf("TestE2E_Clients_Get: id=%d name_len=%d code_len=%d memo_len=%d has_updated_at=%v has_created_at=%v",
		got.ID,
		len(got.Name),
		len(got.Code),
		len(got.Memo),
		got.UpdatedAt != "",
		got.CreatedAt != "",
	)
}

// TestE2E_Clients_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field diff.
// The BOARD API has been observed to ignore the `name` filter across 6
// consecutive milestones (M03/M04/M06/M08/M09/M10) so Search is expected to
// return every client regardless of the query value; this test focuses on
// StrictFieldDiff and artifact collection rather than server-side filtering.
func TestE2E_Clients_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchClientsRaw(ctx, boardapi.ClientSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchClientsRaw: %v", err)
	}

	dumpJSON(t, "clients_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ClientEntity{}); len(diff) > 0 {
		t.Errorf("clients search unmapped fields: %v", diff)
	}

	var items []boardapi.ClientEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Clients_Search: %d items returned", len(items))
}
