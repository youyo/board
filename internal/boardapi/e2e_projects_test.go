//go:build e2e

// E2E tests for /v1/projects against the real BOARD API.
//
// Scope (M13, board-compliance roadmap): Phase E (core business re-verification)
// 2nd milestone — the most complex milestone in the roadmap due to the
// response_group matrix. projects is the 2nd-tier parent resource (just below
// clients) and fans out into estimates/orders/deliveries/invoices/receipts/
// purchase_orders/project_costs.
//
//   - List          (TestE2E_Projects_List)         : 1 req expected
//   - Get           (TestE2E_Projects_Get)          : 2 req expected (list to discover id + get)
//   - Search        (TestE2E_Projects_Search)       : 1 req expected
//   - GetWithGroup  (TestE2E_Projects_GetWithGroup) : 7 req expected (list discovery once, then 6 subtests for estimate/order/delivery/invoice/receipt/all)
//
// Total budget: 11 req (plan cap 15). 1 endpoint 1 execution. No skip on
// 403/429: the BOARD compliance roadmap requires immediate failure so that
// environment or permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, all dependent tests
// skip with "pending re-verification" — tracked in the roadmap "Pending
// Re-verification" table and re-run once data is seeded. For projects a
// zero-item outcome is unlikely on a production account but the skip branch
// is retained for defensive symmetry with M02-M12.
//
// Phase E 2nd note: M09/M10/M11/M12 confirmed core-business resources return
// 200 on Get-by-id. M12 additionally introduced the "Get > List information
// difference" model. M13 differs from M12 in that projects uses an explicit
// response_group parameter to enrich the response with DocumentSummary
// sub-entities (estimate/order/delivery/invoice/receipt), rather than M12's
// automatic expansion. All 6 response_groups are exercised individually so
// the unmapped-field surface of DocumentSummary can be fully characterised.
//
// PII handling: projects contain customer identifiers, project names, codes,
// and internal memos which are commercially sensitive. Raw artifacts are
// written under tmp/e2e-artifacts/ (gitignored) for manual inspection. t.Logf
// output intentionally avoids leaking project names, codes, or memos — only
// lengths, ids, boolean presence, and aggregate counts are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Projects_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Projects_List exercises GET /v1/projects and verifies that every
// JSON key returned by the BOARD API is mapped on ProjectEntity. Aggregate
// statistics (name / code / memo non-empty counts, status distribution) are
// logged without leaking individual values.
func TestE2E_Projects_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListProjectsRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListProjectsRaw: %v", err)
	}

	dumpJSON(t, "projects", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ProjectEntity{}); len(diff) > 0 {
		t.Errorf("projects list unmapped fields: %v", diff)
	}

	var items []boardapi.ProjectEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}

	// Aggregate stats only. Never log individual names, codes, or memos.
	var (
		nameFilled int
		codeFilled int
		memoFilled int
	)
	statusDist := map[string]int{}
	for _, p := range items {
		if p.Name != "" {
			nameFilled++
		}
		if p.Code != "" {
			codeFilled++
		}
		if p.Memo != "" {
			memoFilled++
		}
		statusDist[p.Status]++
	}
	t.Logf("TestE2E_Projects_List: %d items returned", len(items))
	t.Logf("distribution: name_filled=%d/%d code_filled=%d/%d memo_filled=%d/%d",
		nameFilled, len(items),
		codeFilled, len(items),
		memoFilled, len(items),
	)
	// status is a bounded enum (e.g. active/closed/archived); small
	// cardinality is public info, not PII.
	if len(statusDist) <= 20 {
		t.Logf("status distribution (count by value): %v", statusDist)
	} else {
		t.Logf("status has %d unique values (large cardinality; omitting detail)", len(statusDist))
	}
}

// TestE2E_Projects_Get discovers a project id via List and fetches its detail,
// applying strict field diff on the single-object response. Phase E 2nd: 200
// is expected based on M09/M10/M11/M12 precedent; any 404/403 surfaces as a
// Fatal to highlight a regression in core-business Get support.
func TestE2E_Projects_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListProjectsRaw(ctx)
	if err != nil {
		t.Fatalf("ListProjectsRaw (discovery): %v", err)
	}
	var items []boardapi.ProjectEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under "Pending
		// Re-verification" in plans/board-compliance-roadmap.md and re-run
		// once data is seeded.
		t.Skipf("projects list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first project has non-positive ID: %d", id)
	}

	getRaw, err := client.GetProjectRaw(ctx, id)
	if err != nil {
		// Phase E note: M09-M12 confirmed core-business resources return 200
		// on Get-by-id. If M13 regresses to 404 it's a new finding worthy of
		// immediate halt and roadmap capture. 403 is similarly surfaced.
		t.Fatalf("GetProjectRaw(%d): %v", id, err)
	}

	dumpJSON(t, "projects", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ProjectEntity{}); len(diff) > 0 {
		t.Errorf("projects get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.ProjectEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetProject ID mismatch: got=%d want=%d", got.ID, id)
	}

	// Log only lengths and presence booleans. NEVER log the actual name,
	// code, or memo values — project records are commercially sensitive.
	t.Logf("TestE2E_Projects_Get: id=%d client_id=%d name_len=%d code_len=%d status_len=%d start_date_len=%d end_date_len=%d memo_len=%d has_updated_at=%v has_created_at=%v estimate_present=%v order_present=%v delivery_present=%v invoice_present=%v receipt_present=%v",
		got.ID,
		got.ClientID,
		len(got.Name),
		len(got.Code),
		len(got.Status),
		len(got.StartDate),
		len(got.EndDate),
		len(got.Memo),
		got.UpdatedAt != "",
		got.CreatedAt != "",
		got.Estimate != nil,
		got.Order != nil,
		got.Delivery != nil,
		got.Invoice != nil,
		got.Receipt != nil,
	)
}

// TestE2E_Projects_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field
// diff. The BOARD API has been observed to ignore the `name` filter across 7
// consecutive milestones (M03/M04/M06/M08/M09/M10/M12) so Search is expected
// to return every project regardless of the query value; this test focuses on
// StrictFieldDiff and artifact collection rather than server-side filtering.
func TestE2E_Projects_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchProjectsRaw(ctx, boardapi.ProjectSearchParams{
		Name: "zzz_nonexistent_keyword_for_e2e",
	})
	if err != nil {
		t.Fatalf("SearchProjectsRaw: %v", err)
	}

	dumpJSON(t, "projects_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ProjectEntity{}); len(diff) > 0 {
		t.Errorf("projects search unmapped fields: %v", diff)
	}

	var items []boardapi.ProjectEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_Projects_Search: %d items returned", len(items))
}

// TestE2E_Projects_GetWithGroup exercises all 6 response_group variants
// (estimate / order / delivery / invoice / receipt / all) against the same
// project to characterise the DocumentSummary surface for each group. The
// discovery List is shared across all 6 subtests, keeping the total request
// count to 7 (1 list + 6 groups).
//
// Each subtest:
//  1. calls GetProjectWithGroupRaw(id, group)
//  2. writes tmp/e2e-artifacts/projects_rg_<group>_<id>.json
//  3. runs StrictFieldDiff against ProjectEntity — which recursively covers
//     DocumentSummary via the 5 optional pointer fields (Estimate/Order/
//     Delivery/Invoice/Receipt). Unmapped keys surface at both the top level
//     and inside each DocumentSummary.
//  4. logs a boolean presence map of the 5 summary pointers so the caller can
//     tell which documents the project actually owns.
//
// Phase E insight: M12 clients established an implicit "Get > List info diff"
// model. M13 projects uses an explicit response_group matrix. The 6-variant
// pass in a single test is the most efficient way to characterise both the
// matrix semantics and the DocumentSummary schema in a single tmp/ dump set.
func TestE2E_Projects_GetWithGroup(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListProjectsRaw(ctx)
	if err != nil {
		t.Fatalf("ListProjectsRaw (discovery): %v", err)
	}
	var items []boardapi.ProjectEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		t.Skipf("projects list returned 0 items; GetWithGroup pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first project has non-positive ID: %d", id)
	}

	// Canonical order: single-document groups first, then "all".
	groups := []string{"estimate", "order", "delivery", "invoice", "receipt", "all"}
	for _, group := range groups {
		group := group // capture for subtest closure
		t.Run(group, func(t *testing.T) {
			raw, err := client.GetProjectWithGroupRaw(ctx, id, group)
			if err != nil {
				// 403/404/429 must all fail fast — a Phase E regression or a
				// response_group that the API does not accept is worth a
				// roadmap capture.
				t.Fatalf("GetProjectWithGroupRaw(%d, %s): %v", id, group, err)
			}

			dumpJSON(t, fmt.Sprintf("projects_rg_%s", group), id, raw)

			if diff := testhelper.StrictFieldDiff(t, raw, &boardapi.ProjectEntity{}); len(diff) > 0 {
				t.Errorf("projects get_with_group(%d, %s) unmapped fields: %v", id, group, diff)
			}

			var got boardapi.ProjectEntity
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal get_with_group: %v", err)
			}
			if got.ID != id {
				t.Errorf("GetProjectWithGroup ID mismatch: got=%d want=%d", got.ID, id)
			}

			// DocumentSummary presence map. "all" is expected to populate
			// every pointer when the project actually has the document;
			// single-group variants populate only the matching field (or nil
			// if the project has no such document).
			t.Logf("rg=%s: estimate_present=%v order_present=%v delivery_present=%v invoice_present=%v receipt_present=%v",
				group,
				got.Estimate != nil,
				got.Order != nil,
				got.Delivery != nil,
				got.Invoice != nil,
				got.Receipt != nil,
			)

			// When DocumentSummary is populated, log only id and LockFlg
			// (public enum-like) plus length of Total/Tax/TaxWithholding
			// (decimal strings, commercially sensitive; log length only).
			if s := got.Estimate; s != nil {
				t.Logf("rg=%s estimate: id=%d lock_flg=%d total_len=%d tax_len=%d tax_withholding_len=%d message_present=%v",
					group, s.ID, s.LockFlg,
					len(s.Total), len(s.Tax), len(s.TaxWithholding), s.Message != nil)
			}
			if s := got.Order; s != nil {
				t.Logf("rg=%s order: id=%d lock_flg=%d total_len=%d tax_len=%d tax_withholding_len=%d message_present=%v",
					group, s.ID, s.LockFlg,
					len(s.Total), len(s.Tax), len(s.TaxWithholding), s.Message != nil)
			}
			if s := got.Delivery; s != nil {
				t.Logf("rg=%s delivery: id=%d lock_flg=%d total_len=%d tax_len=%d tax_withholding_len=%d message_present=%v",
					group, s.ID, s.LockFlg,
					len(s.Total), len(s.Tax), len(s.TaxWithholding), s.Message != nil)
			}
			if s := got.Invoice; s != nil {
				t.Logf("rg=%s invoice: id=%d lock_flg=%d total_len=%d tax_len=%d tax_withholding_len=%d message_present=%v",
					group, s.ID, s.LockFlg,
					len(s.Total), len(s.Tax), len(s.TaxWithholding), s.Message != nil)
			}
			if s := got.Receipt; s != nil {
				t.Logf("rg=%s receipt: id=%d lock_flg=%d total_len=%d tax_len=%d tax_withholding_len=%d message_present=%v",
					group, s.ID, s.LockFlg,
					len(s.Total), len(s.Tax), len(s.TaxWithholding), s.Message != nil)
			}
		})
	}
}
