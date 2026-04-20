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
// clients と同パターン）。TestE2E_Estimates_GetByDocumentID は M17 docID
// discovery helper のスコープなので本ファイルに残す。

// --- Invoices ---

// TestE2E_Invoices_List verifies that the invoices endpoint is reachable.
// Note: accounts with many invoices may take significant time due to full pagination.
func TestE2E_Invoices_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	invoices, err := client.ListInvoices(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListInvoices")
		skipIfRateLimit(t, err, "ListInvoices")
		t.Fatalf("ListInvoices: %v", err)
	}
	t.Logf("ListInvoices returned %d items", len(invoices))
}

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

// --- Estimates (document path) ---

func TestE2E_Estimates_GetByDocumentID(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Find a project that has an estimate via response_group.
	projects, err := client.ListProjects(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListProjects")
		t.Fatalf("ListProjects: %v", err)
	}

	var documentID int
	for _, p := range projects {
		pw, err := client.GetProjectWithGroup(ctx, p.ID, "estimate")
		if err != nil {
			continue
		}
		if pw.Estimate != nil && pw.Estimate.ID > 0 {
			documentID = pw.Estimate.ID
			t.Logf("Found estimate document ID %d on project %d (%s)", documentID, p.ID, p.Name)
			break
		}
	}
	if documentID == 0 {
		t.Skip("no projects have estimates; cannot test GetEstimate")
	}

	est, err := client.GetEstimate(ctx, documentID)
	if err != nil {
		skipIfNotFound(t, err, "GetEstimate")
		t.Fatalf("GetEstimate(%d): %v", documentID, err)
	}
	requirePositiveID(t, est.ID, "GetEstimate.ID")
	t.Logf("GetEstimate: id=%d title=%q project_id=%d total=%.0f", est.ID, est.Title, est.ProjectID, est.TotalAmount)
}

// --- PurchaseOrders (expenditures path) ---

func TestE2E_PurchaseOrders_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	pos, err := client.ListPurchaseOrders(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListPurchaseOrders")
		t.Fatalf("ListPurchaseOrders: %v", err)
	}
	t.Logf("ListPurchaseOrders returned %d items", len(pos))
}

// --- Payments (expenditure_payments path) ---

func TestE2E_Payments_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	payments, err := client.ListPayments(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListPayments")
		t.Fatalf("ListPayments: %v", err)
	}
	t.Logf("ListPayments returned %d items", len(payments))
}

// --- PurchaseTypes (expenditure_types path) ---
// Moved to e2e_purchase_types_test.go (M06, board-compliance roadmap):
//   TestE2E_PurchaseTypes_List / _Get / _Search with StrictFieldDiff + dumpJSON.
