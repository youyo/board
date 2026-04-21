//go:build e2e

// E2E tests for /v1/contacts against the real BOARD API.
//
// Scope (M10, board-compliance roadmap): Phase D (core business) 2nd milestone.
//   - List   (TestE2E_Contacts_List)   : 1 req expected
//   - Get    (TestE2E_Contacts_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_Contacts_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 10, headroom for paginated contacts). 1 test 1
// endpoint. No skip on 403/429: the BOARD compliance roadmap requires
// immediate failure so that environment or permission issues are not silently
// masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap
// "Pending Re-verification" table and re-run once data is seeded. contacts
// is a core-business resource but optional (some accounts keep clients
// without named contacts), so zero-item branch is realistic.
//
// Phase D 2nd note: M09 client_branches established that core-business
// resources break the master-table Get 404 pattern (200 succeeds) while
// continuing the `name` filter-ignored streak (5th consecutive). Additionally
// M09 surfaced the nested parent entity `client:{...}`. M10 is expected to
// reproduce these patterns — Get 200, name filter ignored, and possibly nested
// `client` and/or `client_branch` envelopes.
//
// 271cba3 validation: this milestone also validates the 2026-04-17 commit
// 271cba3 which added 6 fields to ContactEntity (last_name, first_name,
// honorific_title, department, note, archive_flg) plus the DisplayName()
// method. Fill-rate logging is performed per-field, mirroring the M08 users
// verification pattern.
//
// PII handling: contacts contain personal information (names, emails, phones,
// notes, memos). Raw artifacts are written under tmp/e2e-artifacts/
// (gitignored) for manual inspection. t.Logf output intentionally avoids
// leaking personal values — only lengths, ids, archive flags, and boolean
// branch selectors are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Contacts_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Contacts_List exercises GET /v1/contacts and verifies that every
// JSON key returned by the BOARD API is mapped on ContactEntity. It also logs
// the fill rate of the 6 fields added by 271cba3 (M10 主目的) so the
// commit's correctness can be confirmed against real data.
func TestE2E_Contacts_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListContactsRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListContactsRaw: %v", err)
	}

	dumpJSON(t, "contacts", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ContactEntity{}); len(diff) > 0 {
		t.Errorf("contacts list unmapped fields: %v", diff)
	}

	// Parse minimally to log the observed count for traceability and to
	// compute 271cba3 fill rates. Do NOT log personal values.
	var items []boardapi.ContactEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}

	// 271cba3 validation: count how often each of the 6 added fields is
	// non-empty across all returned contacts. Zero fill-rate on a field is
	// itself a finding (the BOARD API may not populate it in this account).
	var lastNameFilled, firstNameFilled, honorificFilled, departmentFilled, noteFilled int
	var archiveFlg0, archiveFlgNonZero int
	for _, c := range items {
		if c.LastName != "" {
			lastNameFilled++
		}
		if c.FirstName != "" {
			firstNameFilled++
		}
		if c.HonorificTitle != "" {
			honorificFilled++
		}
		if c.Department != "" {
			departmentFilled++
		}
		if c.Note != "" {
			noteFilled++
		}
		if c.ArchiveFlg == 0 {
			archiveFlg0++
		} else {
			archiveFlgNonZero++
		}
	}
	t.Logf("TestE2E_Contacts_List: %d items returned", len(items))
	t.Logf("271cba3 fill rate: last_name=%d/%d first_name=%d/%d honorific_title=%d/%d department=%d/%d note=%d/%d archive_flg[0=%d,non0=%d]",
		lastNameFilled, len(items),
		firstNameFilled, len(items),
		honorificFilled, len(items),
		departmentFilled, len(items),
		noteFilled, len(items),
		archiveFlg0, archiveFlgNonZero,
	)
}

// TestE2E_Contacts_Get discovers a contact id via List and fetches its
// detail, applying strict field diff on the single-object response. Logs the
// DisplayName() branch selector (Name path vs LastName+FirstName path) per
// the M08 users validation pattern.
func TestE2E_Contacts_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListContactsRaw(ctx)
	if err != nil {
		t.Fatalf("ListContactsRaw (discovery): %v", err)
	}
	var items []boardapi.ContactEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("contacts list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first contact has non-positive ID: %d", id)
	}

	getRaw, err := client.GetContactRaw(ctx, id)
	if err != nil {
		// Phase D note: M09 confirmed core-business resources return 200 on
		// Get-by-id (unlike master-tables which 404 consistently). If M10
		// regresses to 404 it's a new finding worthy of immediate halt and
		// roadmap capture. 403 is similarly surfaced.
		t.Fatalf("GetContactRaw(%d): %v", id, err)
	}

	dumpJSON(t, "contacts", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ContactEntity{}); len(diff) > 0 {
		t.Errorf("contacts get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.ContactEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetContact ID mismatch: got=%d want=%d", got.ID, id)
	}

	// DisplayName() branch trace (M40: Name field removed, always LastName+FirstName path).
	displayName := got.DisplayName()

	// Helper to get length of *string field (0 for nil).
	ptrLen := func(s *string) int {
		if s == nil {
			return 0
		}
		return len(*s)
	}

	// Log only lengths, ids, and booleans for traceability. NEVER log the
	// actual personal data (last_name/first_name/email/note/title/department).
	t.Logf("TestE2E_Contacts_Get: id=%d client_id=%d archive_flg=%d last_name_len=%d first_name_len=%d honorific_title_len=%d title_len=%d department_len=%d email_len=%d note_len=%d display_name_len=%d",
		got.ID,
		got.ClientID(),
		got.ArchiveFlg,
		len(got.LastName),
		len(got.FirstName),
		len(got.HonorificTitle),
		ptrLen(got.Title),
		ptrLen(got.Department),
		ptrLen(got.Email),
		ptrLen(got.Note),
		len(displayName),
	)
}

// TestE2E_Contacts_Search exercises Search with a non-matching name and
// verifies that the (possibly empty) JSON array still passes strict field
// diff. Phase D note: M02-M09 showed 5 consecutive resources ignoring the
// `name` query parameter. If M10 makes this the 6th consecutive, the BOARD
// API-wide "filter ignored" assumption crystallizes; if contacts honors the
// filter, the streak breaks. Either outcome is documented in the roadmap.
// StrictFieldDiff remains meaningful against whatever is returned.
func TestE2E_Contacts_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchContactsRaw(ctx, boardapi.ContactSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchContactsRaw: %v", err)
	}

	dumpJSON(t, "contacts_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ContactEntity{}); len(diff) > 0 {
		t.Errorf("contacts search unmapped fields: %v", diff)
	}

	var items []boardapi.ContactEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Contacts_Search: %d items returned", len(items))
}
