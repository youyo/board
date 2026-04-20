//go:build e2e

// E2E tests for /v1/project_costs against the real BOARD API.
//
// Scope (M11, board-compliance roadmap): Phase D (core business) 3rd and
// final milestone.
//   - List   (TestE2E_ProjectCosts_List)   : 1 req expected
//   - Get    (TestE2E_ProjectCosts_Get)    : 2 req expected (list to discover id + get)
//   - Search (TestE2E_ProjectCosts_Search) : 1 req expected
//
// Total budget: 4 req (plan cap 10). 1 test 1 endpoint. No skip on 403/429:
// the BOARD compliance roadmap requires immediate failure so that environment
// or permission issues are not silently masked.
//
// Data-dependent skip: when the List response is empty, the Get test skips
// with "pending re-verification" — tracked in the roadmap
// "Pending Re-verification" table and re-run once data is seeded.
// project_costs depends on projects being registered, so zero-item is realistic
// for accounts without any project cost records.
//
// Phase D 3rd note: M09 client_branches and M10 contacts established that
// core-business resources (1) return 200 on Get-by-id (unlike master tables
// which 404 consistently) and (2) embed a nested parent entity
// `client:{id,name,name_disp,custom_no}`. M11 is expected to reproduce the
// Get 200 outcome, and — if the pattern generalizes — surface a nested
// `project:{...}` envelope since project_costs is the child of projects.
// Either presence or absence is documented.
//
// PII handling: project_costs may include contracted amounts and project names
// which are commercially sensitive. Raw artifacts are written under
// tmp/e2e-artifacts/ (gitignored) for manual inspection. t.Logf output
// intentionally avoids leaking personal or financial values — only lengths,
// ids, nonzero counts, aggregate stats (sum/max/min), and enum uniqueness are
// logged. Individual names, memos, and individual amounts are never logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_ProjectCosts_List ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_ProjectCosts_List exercises GET /v1/project_costs and verifies that
// every JSON key returned by the BOARD API is mapped on ProjectCostEntity. It
// also logs distribution stats for Amount / CostType / Memo so the real-API
// population can be understood without leaking individual values.
func TestE2E_ProjectCosts_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListProjectCostsRaw(ctx)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("ListProjectCostsRaw: %v", err)
	}

	dumpJSON(t, "project_costs", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ProjectCostEntity{}); len(diff) > 0 {
		t.Errorf("project_costs list unmapped fields: %v", diff)
	}

	var items []boardapi.ProjectCostEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}

	// Aggregate stats only. Never log individual names, memos, or amounts.
	var (
		amountNonZero int
		amountSum     float64
		amountMin     float64
		amountMax     float64
		memoFilled    int
		nameFilled    int
	)
	costTypes := map[string]int{}
	for i, pc := range items {
		if pc.Amount != 0 {
			amountNonZero++
			if i == 0 || pc.Amount < amountMin {
				amountMin = pc.Amount
			}
			if pc.Amount > amountMax {
				amountMax = pc.Amount
			}
		}
		amountSum += pc.Amount
		if pc.Memo != "" {
			memoFilled++
		}
		if pc.Name != "" {
			nameFilled++
		}
		costTypes[pc.CostType]++
	}
	t.Logf("TestE2E_ProjectCosts_List: %d items returned", len(items))
	t.Logf("distribution: name_filled=%d/%d memo_filled=%d/%d amount_nonzero=%d/%d amount_sum=%.2f amount_min=%.2f amount_max=%.2f",
		nameFilled, len(items),
		memoFilled, len(items),
		amountNonZero, len(items),
		amountSum, amountMin, amountMax,
	)
	// cost_type is expected to be an enum (e.g. labor/material/outsourcing/
	// other); small cardinality is public info, not PII.
	if len(costTypes) <= 10 {
		t.Logf("cost_type distribution (count by value): %v", costTypes)
	} else {
		t.Logf("cost_type has %d unique values (large cardinality; omitting detail)", len(costTypes))
	}
}

// TestE2E_ProjectCosts_Get discovers a project cost id via List and fetches
// its detail, applying strict field diff on the single-object response. Phase
// D 3rd: 200 is expected based on M09/M10 precedent; any 404/403 surfaces as a
// Fatal to highlight a regression in core-business Get support.
func TestE2E_ProjectCosts_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	listRaw, err := client.ListProjectCostsRaw(ctx)
	if err != nil {
		t.Fatalf("ListProjectCostsRaw (discovery): %v", err)
	}
	var items []boardapi.ProjectCostEntity
	if err := json.Unmarshal(listRaw, &items); err != nil {
		t.Fatalf("unmarshal list for discovery: %v", err)
	}
	if len(items) == 0 {
		// Data-dependent skip: distinct from the roadmap "no skip on 403/429"
		// rule, which targets rate-limit / permission failures. Zero data
		// means Get cannot be exercised; tracked under
		// "Pending Re-verification" in plans/board-compliance-roadmap.md and
		// re-run once data is seeded.
		t.Skipf("project_costs list returned 0 items; Get pending re-verification (see roadmap Pending Re-verification)")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first project_cost has non-positive ID: %d", id)
	}

	getRaw, err := client.GetProjectCostRaw(ctx, id)
	if err != nil {
		// Phase D note: M09/M10 confirmed core-business resources return 200
		// on Get-by-id. If M11 regresses to 404 it's a new finding worthy of
		// immediate halt and roadmap capture. 403 is similarly surfaced.
		t.Fatalf("GetProjectCostRaw(%d): %v", id, err)
	}

	dumpJSON(t, "project_costs", id, getRaw)

	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ProjectCostEntity{}); len(diff) > 0 {
		t.Errorf("project_costs get(%d) unmapped fields: %v", id, diff)
	}

	var got boardapi.ProjectCostEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetProjectCost ID mismatch: got=%d want=%d", got.ID, id)
	}

	// Log only lengths, ids, booleans, and nonzero flags for traceability.
	// NEVER log the actual name/memo/amount values — project_costs contain
	// commercially sensitive financial data.
	t.Logf("TestE2E_ProjectCosts_Get: id=%d project_id=%d name_len=%d cost_type_len=%d amount_nonzero=%v memo_len=%d has_updated_at=%v has_created_at=%v",
		got.ID,
		got.ProjectID,
		len(got.Name),
		len(got.CostType),
		got.Amount != 0,
		len(got.Memo),
		got.UpdatedAt != "",
		got.CreatedAt != "",
	)
}

// TestE2E_ProjectCosts_Search exercises Search without a ProjectID filter
// (equivalent to List in the current SearchParams surface) and verifies that
// the returned array still passes strict field diff. Unlike M10 contacts
// which probed the `name` filter (ignored 6 times in a row), project_costs
// only exposes `project_id`, which is a numeric hierarchical filter — a
// different semantic from the name-ignored BOARD API-wide pattern. A
// dedicated ProjectID-filter E2E is deferred to a follow-up milestone.
func TestE2E_ProjectCosts_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.SearchProjectCostsRaw(ctx, boardapi.ProjectCostSearchParams{})
	if err != nil {
		t.Fatalf("SearchProjectCostsRaw: %v", err)
	}

	dumpJSON(t, "project_costs_search", 0, raw)

	if diff := testhelper.StrictFieldDiff(t, raw, &[]boardapi.ProjectCostEntity{}); len(diff) > 0 {
		t.Errorf("project_costs search unmapped fields: %v", diff)
	}

	var items []boardapi.ProjectCostEntity
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	t.Logf("TestE2E_ProjectCosts_Search: %d items returned", len(items))
}
