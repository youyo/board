//go:build e2e

// E2E tests for /v1/groups against the real BOARD API.
//
// Scope (M07, board-compliance roadmap):
//   - List (TestE2E_Groups_List) : 1 req expected
//   - Get  (TestE2E_Groups_Get)  : 2 req expected (list to discover id + get)
//
// Total budget: 3 req (plan cap 5). 1 test 1 endpoint. No skip on 403/429: the
// BOARD compliance roadmap requires immediate failure so that environment or
// permission issues are not silently masked.
//
// Search is intentionally NOT covered by M07: the roadmap defines M07 as
// "Get (with existing List as prerequisite) + strict field diff (GroupEntity)"
// only. Any future Search verification belongs to a separate milestone.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded.
//
// Each test writes the raw response body to tmp/e2e-artifacts/ via dumpJSON
// (gitignored) and runs testhelper.StrictFieldDiff against GroupEntity to
// detect unmapped keys.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Groups_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Groups_List exercises GET /v1/groups and verifies that every JSON
// key returned by the BOARD API is mapped on GroupEntity.
func TestE2E_Groups_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListGroupsRaw(ctx, boardapi.GroupListOptions{})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListGroupsRaw: %v", err)
	}

	dumpJSON(t, "groups", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.GroupEntity{}); len(diff) > 0 {
		t.Errorf("groups list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability.
	var items []boardapi.GroupEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_Groups_List: %d items returned", len(items))
}

// TestE2E_Groups_Get discovers a group id via List and fetches its detail,
// applying strict field diff on the single-object response.
func TestE2E_Groups_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListGroupsRaw(ctx, boardapi.GroupListOptions{})
	if err != nil {
		t.Fatalf("ListGroupsRaw (discovery): %v", err)
	}
	var items []boardapi.GroupEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data means
		// Get cannot be exercised; tracked under "Pending Re-verification" in
		// plans/board-compliance-roadmap.md and re-run once data is seeded.
		t.Skipf("groups list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first group has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetGroupRaw(ctx, id)
	if err != nil {
		t.Fatalf("GetGroupRaw(%d): %v", id, err)
	}

	dumpJSON(t, "groups", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.GroupEntity{}); len(diff) > 0 {
		t.Errorf("groups get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.GroupEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetGroup ID mismatch: got=%d want=%d", got.ID, id)
	}
	t.Logf("TestE2E_Groups_Get: id=%d name=%q", got.ID, got.Name)
}
