//go:build e2e

// E2E tests for /v1/users against the real BOARD API.
//
// Scope (M08, board-compliance roadmap):
//   - List   (TestE2E_Users_List)   : 1 req expected
//   - Get    (TestE2E_Users_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_Users_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 5). 1 test 1 endpoint. No skip on 403/429: the
// BOARD compliance roadmap requires immediate failure so that environment or
// permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips with
// "pending re-verification" — tracked in the roadmap "Pending Re-verification"
// table and re-run once data is seeded. For users this path is unlikely because
// the authenticated caller themself is expected to be returned, but the skip
// branch is retained for defensive symmetry with M02-M07.
//
// Each test writes the raw response body to tmp/e2e-artifacts/ via dumpJSON
// (gitignored) and runs testhelper.StrictFieldDiff against UserEntity to
// detect unmapped keys. 271cba3 (2026-04-17) added last_name / first_name /
// role_id / role_name / last_sign_in_at / valid_flg to UserEntity; M08 is the
// milestone that verifies the fix against the real API.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Users_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Users_List exercises GET /v1/users and verifies that every JSON key
// returned by the BOARD API is mapped on UserEntity.
func TestE2E_Users_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListUsersRaw(ctx, boardapi.UserListOptions{})
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListUsersRaw: %v", err)
	}

	dumpJSON(t, "users", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.UserEntity{}); len(diff) > 0 {
		t.Errorf("users list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability.
	var items []boardapi.UserEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	t.Logf("TestE2E_Users_List: %d items returned", len(items))
}

// TestE2E_Users_Get discovers a user id via List and fetches its detail,
// applying strict field diff on the single-object response.
func TestE2E_Users_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, _, err := client.ListUsersRaw(ctx, boardapi.UserListOptions{})
	if err != nil {
		t.Fatalf("ListUsersRaw (discovery): %v", err)
	}
	var items []boardapi.UserEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data means
		// Get cannot be exercised; tracked under "Pending Re-verification" in
		// plans/board-compliance-roadmap.md and re-run once data is seeded.
		t.Skipf("users list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first user has non-positive ID: %d", id)
	}

	getRaw, _, err := client.GetUserRaw(ctx, id)
	if err != nil {
		t.Fatalf("GetUserRaw(%d): %v", id, err)
	}

	dumpJSON(t, "users", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.UserEntity{}); len(diff) > 0 {
		t.Errorf("users get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.UserEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetUser ID mismatch: got=%d want=%d", got.ID, id)
	}
	// DisplayName() is the 271cba3 helper; log without leaking the raw value
	// (artifact JSON is still dumped to tmp/ for manual inspection).
	t.Logf("TestE2E_Users_Get: id=%d display_name_len=%d role_id=%d valid_flg=%d has_last_sign_in=%t",
		got.ID, len(got.DisplayName()), got.RoleID, got.ValidFlg, got.LastSignInAt != "")
}

// TestE2E_Users_Search exercises Search with a non-matching name and verifies
// that the (possibly empty) JSON array still passes strict field diff. Even if
// the BOARD API ignores the name filter (as observed for project_types /
// payment_terms / purchase_types), the strict field diff remains meaningful
// against whatever is returned.
func TestE2E_Users_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, _, err := client.ListUsersRaw(ctx, boardapi.UserListOptions{
		NameCont: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("ListUsersRaw (search): %v", err)
	}

	dumpJSON(t, "users_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.UserEntity{}); len(diff) > 0 {
		t.Errorf("users search unmapped fields: %v", diff)
	}

	var items []boardapi.UserEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Users_Search: %d items returned", len(items))
}
