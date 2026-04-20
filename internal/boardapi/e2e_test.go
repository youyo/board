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

	"github.com/youyo/board/internal/boardapi"
)

// --- Clients ---

func TestE2E_Clients_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	clients, err := client.ListClients(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListClients")
		t.Fatalf("ListClients: %v", err)
	}
	t.Logf("ListClients returned %d items", len(clients))
}

func TestE2E_Clients_GetByID(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	clients, err := client.ListClients(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListClients")
		t.Fatalf("ListClients: %v", err)
	}
	if len(clients) == 0 {
		t.Skip("no clients available for GetByID test")
	}

	got, err := client.GetClient(ctx, clients[0].ID)
	if err != nil {
		skipIfNotFound(t, err, "GetClient")
		t.Fatalf("GetClient(%d): %v", clients[0].ID, err)
	}
	requirePositiveID(t, got.ID, "GetClient.ID")
	requireNonEmpty(t, got.Name, "GetClient.Name")
	if got.ID != clients[0].ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, clients[0].ID)
	}
}

func TestE2E_Clients_Search(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	_, err := client.SearchClients(ctx, boardapi.ClientSearchParams{})
	if err != nil {
		skipIfNotFound(t, err, "SearchClients")
		t.Fatalf("SearchClients: %v", err)
	}
}

// --- Users ---

func TestE2E_Users_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	users, err := client.ListUsers(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListUsers")
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) < 1 {
		t.Errorf("ListUsers: expected at least 1 user, got %d", len(users))
	}
	t.Logf("ListUsers returned %d items", len(users))
}

func TestE2E_Users_GetByID(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	users, err := client.ListUsers(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListUsers")
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) == 0 {
		t.Skip("no users available for GetByID test")
	}

	got, err := client.GetUser(ctx, users[0].ID)
	if err != nil {
		// BOARD API may not support individual user GET for all account types.
		skipIfNotFound(t, err, "GetUser")
		t.Fatalf("GetUser(%d): %v", users[0].ID, err)
	}
	requirePositiveID(t, got.ID, "GetUser.ID")
	requireNonEmpty(t, got.DisplayName(), "GetUser.DisplayName")
	if got.ID != users[0].ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, users[0].ID)
	}
}

// --- Groups ---

func TestE2E_Groups_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	groups, err := client.ListGroups(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListGroups")
		t.Fatalf("ListGroups: %v", err)
	}
	t.Logf("ListGroups returned %d items", len(groups))
}

// --- Projects ---

func TestE2E_Projects_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	projects, err := client.ListProjects(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListProjects")
		t.Fatalf("ListProjects: %v", err)
	}
	t.Logf("ListProjects returned %d items", len(projects))
}

func TestE2E_Projects_GetByID(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	projects, err := client.ListProjects(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListProjects")
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) == 0 {
		t.Skip("no projects available for GetByID test")
	}

	got, err := client.GetProject(ctx, projects[0].ID)
	if err != nil {
		skipIfNotFound(t, err, "GetProject")
		t.Fatalf("GetProject(%d): %v", projects[0].ID, err)
	}
	requirePositiveID(t, got.ID, "GetProject.ID")
	if got.ID != projects[0].ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, projects[0].ID)
	}
}

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

// --- Projects (response_group) ---

func TestE2E_Projects_GetWithGroup(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	projects, err := client.ListProjects(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListProjects")
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) == 0 {
		t.Skip("no projects available")
	}

	got, err := client.GetProjectWithGroup(ctx, projects[0].ID, "estimate")
	if err != nil {
		skipIfNotFound(t, err, "GetProjectWithGroup")
		t.Fatalf("GetProjectWithGroup(%d, estimate): %v", projects[0].ID, err)
	}
	requirePositiveID(t, got.ID, "GetProjectWithGroup.ID")
	if got.Estimate != nil {
		t.Logf("Project %d has estimate: ID=%d", got.ID, got.Estimate.ID)
	} else {
		t.Logf("Project %d has no estimate", got.ID)
	}
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

// --- Vendors (payees path) ---

func TestE2E_Vendors_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	vendors, err := client.ListVendors(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListVendors")
		t.Fatalf("ListVendors: %v", err)
	}
	t.Logf("ListVendors returned %d items", len(vendors))
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

func TestE2E_PurchaseTypes_List(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	pts, err := client.ListPurchaseTypes(ctx)
	if err != nil {
		skipIfNotFound(t, err, "ListPurchaseTypes")
		t.Fatalf("ListPurchaseTypes: %v", err)
	}
	t.Logf("ListPurchaseTypes returned %d items", len(pts))
}
