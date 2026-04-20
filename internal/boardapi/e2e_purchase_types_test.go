//go:build e2e

// E2E tests for /v1/expenditure_types (BOARD API internal path for
// purchase_types) against the real BOARD API.
//
// Scope (M06, board-compliance roadmap):
//   - List  (TestE2E_PurchaseTypes_List)    : 1 req expected
//   - Get   (TestE2E_PurchaseTypes_Get)     : 2 req expected (list to discover id + get)
//   - Search(TestE2E_PurchaseTypes_Search)  : 1 req expected
//
// Total budget: 4 req (plan cap 5). 1 test 1 endpoint. No skip on 403/429: the
// BOARD compliance roadmap requires immediate failure so that environment or
// permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips with
// "pending re-verification" — tracked in the roadmap "Pending Re-verification"
// table and re-run once data is seeded.
//
// Each test writes the raw response body to tmp/e2e-artifacts/ via dumpJSON
// (gitignored) and runs testhelper.StrictFieldDiff against PurchaseTypeEntity
// to detect unmapped keys.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_PurchaseTypes_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_PurchaseTypes_List exercises GET /v1/expenditure_types and verifies
// that every JSON key returned by the BOARD API is mapped on PurchaseTypeEntity.
func TestE2E_PurchaseTypes_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListPurchaseTypesRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListPurchaseTypesRaw: %v", err)
	}

	dumpJSON(t, "purchase_types", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.PurchaseTypeEntity{}); len(diff) > 0 {
		t.Errorf("purchase_types list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability.
	var items []boardapi.PurchaseTypeEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_PurchaseTypes_List: %d items returned", len(items))
}

// TestE2E_PurchaseTypes_Get discovers a purchase_type id via List and fetches
// its detail, applying strict field diff on the single-object response.
func TestE2E_PurchaseTypes_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListPurchaseTypesRaw(ctx)
	if err != nil {
		t.Fatalf("ListPurchaseTypesRaw (discovery): %v", err)
	}
	var items []boardapi.PurchaseTypeEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data means
		// Get cannot be exercised; tracked under "Pending Re-verification" in
		// plans/board-compliance-roadmap.md and re-run once data is seeded.
		t.Skipf("purchase_types list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first purchase_type has non-positive ID: %d", id)
	}

	getRaw, err := client.GetPurchaseTypeRaw(ctx, id)
	if err != nil {
		t.Fatalf("GetPurchaseTypeRaw(%d): %v", id, err)
	}

	dumpJSON(t, "purchase_types", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.PurchaseTypeEntity{}); len(diff) > 0 {
		t.Errorf("purchase_types get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.PurchaseTypeEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetPurchaseType ID mismatch: got=%d want=%d", got.ID, id)
	}
	t.Logf("TestE2E_PurchaseTypes_Get: id=%d name=%q", got.ID, got.Name)
}

// TestE2E_PurchaseTypes_Search exercises Search with a non-matching name and
// verifies that the (possibly empty) JSON array still passes strict field diff.
// Even if the BOARD API ignores the name filter, the strict field diff remains
// meaningful against whatever is returned.
func TestE2E_PurchaseTypes_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchPurchaseTypesRaw(ctx, boardapi.PurchaseTypeSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchPurchaseTypesRaw: %v", err)
	}

	dumpJSON(t, "purchase_types_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.PurchaseTypeEntity{}); len(diff) > 0 {
		t.Errorf("purchase_types search unmapped fields: %v", diff)
	}

	var items []boardapi.PurchaseTypeEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_PurchaseTypes_Search: %d items returned", len(items))
}
