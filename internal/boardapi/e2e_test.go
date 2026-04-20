//go:build e2e

// E2E tests for the boardapi package against the real BOARD API.
// Estimated API calls per full run: ~12 (0.4% of the 3000/day rate limit).
//
// Resources with 404 (not enabled for this account) are automatically skipped.
//
// Usage:
//
//	BOARD_API_KEY=<key> BOARD_API_TOKEN=<token> go test -tags e2e -v -count=1 ./internal/boardapi/ -run TestE2E
//
// M02 以降の新規 E2E の書き方:
//
//   - 実 API から取得した raw JSON を dumpJSON(t, "<resource>", id, raw) で
//     tmp/e2e-artifacts/ に残す（.gitignore 済み）。
//   - testhelper.StrictFieldDiff(t, raw, &TargetEntity{}) を必ず呼び、未マップ
//     フィールドが 1 件でもあれば t.Errorf で失敗させる。
//   - List では id=0、Get/Search では対象リソースの ID を第二引数に使う。
//
// これにより「Go struct に存在しないフィールドが BOARD API 側に追加された」
// 状況を E2E で早期検知できる。

package boardapi_test

import (
	"context"
	"testing"
)

// --- Clients ---
// TestE2E_Clients_List / TestE2E_Clients_GetByID / TestE2E_Clients_Search は
// M12 で厳格フィールド突合付きの版に一本化したため e2e_clients_test.go へ
// 移動（M06/M07/M08 と同パターン）。TestE2E_Clients_ListPage はページング
// 検証として独立価値があるので本ファイルに残す。

// --- Users ---
// TestE2E_Users_List / TestE2E_Users_GetByID は M08 で厳格フィールド突合付き
// の版に一本化したため e2e_users_test.go へ移動（M07 groups と同形）。

// --- Groups ---
// TestE2E_Groups_List は M07 で厳格フィールド突合付きの版に一本化したため
// e2e_groups_test.go へ移動。

// --- Projects ---
// TestE2E_Projects_List / TestE2E_Projects_GetByID / TestE2E_Projects_GetWithGroup
// は M13 で厳格フィールド突合付き（List/Get/Search + GetWithGroup 全 6
// response_group）の版に一本化したため e2e_projects_test.go へ移動（M12
// clients と同パターン）。

// --- Estimates ---
// TestE2E_Estimates_Get は M18 で M17 helper 経由の厳格突合版に一本化したため
// e2e_estimates_test.go へ移動。

// --- Invoices ---
// TestE2E_Invoices_List / _Get / _Search は M22 で厳格フィールド突合付きの版に
// 一本化したため e2e_invoices_test.go へ移動（M16 vendors と同パターン）。

// --- Clients (pagination) ---

func TestE2E_Clients_ListPage(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	pr, err := client.ListClientsPage(ctx, 1, 5)
	if err != nil {
		skipIfNotFound(t, err, "ListClientsPage")
		t.Fatalf("ListClientsPage: %v", err)
	}
	if pr.TotalCount <= 0 {
		t.Errorf("TotalCount expected > 0, got %d", pr.TotalCount)
	}
	if pr.Page != 1 {
		t.Errorf("Page expected 1, got %d", pr.Page)
	}
	if len(pr.Items) > 5 {
		t.Errorf("Items expected <= 5, got %d", len(pr.Items))
	}
	t.Logf("ListClientsPage: total=%d page=%d items=%d", pr.TotalCount, pr.Page, len(pr.Items))
}

// --- PurchaseOrders (expenditures path) ---
// TestE2E_PurchaseOrders_List / _Get / _Search は M23 で厳格フィールド突合付きの版に
// 一本化したため e2e_purchase_orders_test.go へ移動（M16 vendors と同パターン）。

// --- Payments (expenditure_payments path) ---
// TestE2E_Payments_List / _Get / _Search は M24 で厳格フィールド突合付きの版に
// 一本化したため e2e_payments_test.go へ移動（M16 vendors と同パターン）。

// --- PurchaseTypes (expenditure_types path) ---
// Moved to e2e_purchase_types_test.go (M06, board-compliance roadmap):
//   TestE2E_PurchaseTypes_List / _Get / _Search with StrictFieldDiff + dumpJSON.
