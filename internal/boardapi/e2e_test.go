//go:build e2e

// E2E tests for the boardapi package against the real BOARD API.
// Estimated API calls per full run: ~12 (0.4% of the 3000/day rate limit).
//
// Resources with 404 (not enabled for this account) are automatically skipped.
//
// Usage:
//
//	BOARD_API_KEY=<key> BOARD_API_TOKEN=<token> go test -tags e2e -v -count=1 ./internal/boardapi/ -run TestE2E

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
	requireNonEmpty(t, got.Name, "GetUser.Name")
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
		t.Fatalf("ListInvoices: %v", err)
	}
	t.Logf("ListInvoices returned %d items", len(invoices))
}
