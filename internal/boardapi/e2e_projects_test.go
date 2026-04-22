//go:build e2e

// E2E tests for /v1/projects against the real BOARD API.
//
// Scope (M44, board-compliance roadmap): Phase K — ProjectEntity 全面再設計後の
// 実 API との厳格フィールド突合。M13 版を M44 の 72 フィールド + DocumentSummary 拡張に対応。
//
//   - List          (TestE2E_Projects_List)          : 1 req expected
//   - Get           (TestE2E_Projects_Get)           : 2 req expected (list discovery + get)
//   - Search        (TestE2E_Projects_Search)        : 1 req expected
//   - GetWithGroup  (TestE2E_Projects_GetWithGroup)  : 7 req expected (1 list + 6 rg subtests)
//
// Total budget: 11 req (plan cap 15).
//
// Usage (single-shot per test):
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
// JSON key returned by the BOARD API is mapped on ProjectEntity.
// M44: 旧フィールド（name_filled/code_filled/memo_filled/status）を
// 新フィールド（order_status_name/delivery_status_name/in_house_memo）に更新。
func TestE2E_Projects_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	raw, err := client.ListProjectsRaw(ctx)
	if err != nil {
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

	// Aggregate stats only — never log individual project names or sensitive values.
	var (
		nameFilled          int
		orderStatusFilled   int
		deliveryStatusFilled int
		clientFilled        int
	)
	orderStatusDist := map[string]int{}
	deliveryStatusDist := map[string]int{}
	for _, p := range items {
		if p.Name != "" {
			nameFilled++
		}
		if p.OrderStatusName != "" {
			orderStatusFilled++
		}
		if p.DeliveryStatusName != "" {
			deliveryStatusFilled++
		}
		if p.Client != nil {
			clientFilled++
		}
		orderStatusDist[p.OrderStatusName]++
		deliveryStatusDist[p.DeliveryStatusName]++
	}
	t.Logf("TestE2E_Projects_List: %d items returned", len(items))
	t.Logf("distribution: name_filled=%d/%d order_status_filled=%d/%d delivery_status_filled=%d/%d client_filled=%d/%d",
		nameFilled, len(items),
		orderStatusFilled, len(items),
		deliveryStatusFilled, len(items),
		clientFilled, len(items),
	)
	// order_status_name / delivery_status_name は bounded enum; 小カーディナリティはパブリック情報
	if len(orderStatusDist) <= 20 {
		t.Logf("order_status distribution: %v", orderStatusDist)
	}
	if len(deliveryStatusDist) <= 20 {
		t.Logf("delivery_status distribution: %v", deliveryStatusDist)
	}
}

// TestE2E_Projects_Get discovers a project ID via List and fetches its detail,
// applying strict field diff on the 72-field single-object response.
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
		t.Skipf("projects list returned 0 items; Get pending re-verification")
	}

	id := items[0].ID
	if id <= 0 {
		t.Fatalf("first project has non-positive ID: %d", id)
	}

	getRaw, err := client.GetProjectRaw(ctx, id)
	if err != nil {
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

	// Log only lengths and presence booleans — project records are commercially sensitive.
	t.Logf("TestE2E_Projects_Get: id=%d name_len=%d order_status=%d delivery_status=%d"+
		" has_client=%v client_id=%d has_user=%v has_hubspot=%v"+
		" contract_start=%v contract_end=%v has_updated_at=%v",
		got.ID,
		len(got.Name),
		got.OrderStatus,
		got.DeliveryStatus,
		got.Client != nil,
		func() int {
			if got.Client != nil {
				return got.Client.ID
			}
			return 0
		}(),
		got.User != nil,
		got.Hubspot != nil,
		got.ContractStartDate != nil,
		got.ContractEndDate != nil,
		got.UpdatedAt != "",
	)
}

// TestE2E_Projects_Search exercises Search with a non-matching name and
// verifies that the (possibly full) JSON array still passes strict field diff.
// BOARD API は name フィルタを無視することがコンプライアンスロードマップで確認済み。
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
// project. M44: DocumentSummary 拡張 + Deliveries/Invoices/Receipts 配列化 に対応。
//
// 各サブテスト:
//  1. GetProjectWithGroupRaw(id, group) 呼び出し
//  2. tmp/e2e-artifacts/projects_rg_<group>_<id>.json に書き込み
//  3. StrictFieldDiff を ProjectEntity に適用（DocumentSummary も再帰的にカバー）
//  4. estimate/order 単数 / deliveries/invoices/receipts 配列の presence ログ
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
		t.Skipf("projects list returned 0 items; GetWithGroup pending re-verification")
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

			// M44: 単数形 Invoice/Delivery/Receipt 廃止、配列形式で確認
			t.Logf("rg=%s: estimate_present=%v order_present=%v deliveries_count=%d invoices_count=%d receipts_count=%d project_costs_len=%d",
				group,
				got.Estimate != nil,
				got.Order != nil,
				len(got.Deliveries),
				len(got.Invoices),
				len(got.Receipts),
				len(got.ProjectCosts),
			)

			// DocumentSummary presence の詳細ログ（ID, LockFlg のみ — 金額は商業機密）
			if s := got.Estimate; s != nil {
				t.Logf("rg=%s estimate: id=%d lock_flg=%d total_len=%d details_count=%d valid_period_present=%v",
					group, s.ID, s.LockFlg, len(s.Total), len(s.Details), s.ValidPeriod != nil)
			}
			if s := got.Order; s != nil {
				t.Logf("rg=%s order: id=%d lock_flg=%d total_len=%d details_count=%d",
					group, s.ID, s.LockFlg, len(s.Total), len(s.Details))
			}
			for i, d := range got.Deliveries {
				t.Logf("rg=%s deliveries[%d]: id=%d delivery_date_present=%v delivery_place_present=%v details_count=%d",
					group, i, d.ID, d.DeliveryDate != nil, d.DeliveryPlace != nil, len(d.Details))
			}
			for i, inv := range got.Invoices {
				t.Logf("rg=%s invoices[%d]: id=%d invoice_date_present=%v payment_limit_present=%v details_count=%d",
					group, i, inv.ID, inv.InvoiceDate != nil, inv.PaymentLimitDate != nil, len(inv.Details))
			}
			for i, r := range got.Receipts {
				t.Logf("rg=%s receipts[%d]: id=%d receipt_date_present=%v details_count=%d",
					group, i, r.ID, r.ReceiptDate != nil, len(r.Details))
			}
		})
	}
}
