//go:build e2e

// E2E tests for the service/find package using real BOARD API data.
// These tests verify cross-resource resolution (Client + Branches + Contacts, Project + Client, etc.)
// Estimated API calls per full run: ~15 (0.5% of the 3000/day rate limit).
//
// Usage:
//
//	BOARD_API_KEY=<key> BOARD_API_TOKEN=<token> go test -tags e2e -v -count=1 ./internal/service/find/ -run TestE2E

package find_test

import (
	"context"
	"testing"

	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// e2eOpts returns ReadOptions suitable for E2E tests.
// No refresh flags: GetByID fetches a single entity on cache miss (fast).
// List/Search will do an implicit full fetch on empty cache (unavoidable first-time cost).
func e2eOpts() repository.ReadOptions {
	return repository.ReadOptions{}
}

// --- FindClient ---

func TestE2E_FindClient_ByName(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// Get a real client name from the API.
	clients, err := api.ListClients(ctx)
	if err != nil || len(clients) == 0 {
		t.Skip("no clients available")
	}
	targetName := clients[0].Name

	results, err := svc.FindClient(ctx, find.FindClientQuery{
		Name:  targetName,
		Limit: 10,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindClient(Name=%q): %v", targetName, err)
	}
	if len(results) == 0 {
		t.Errorf("FindClient(Name=%q): expected >= 1 result, got 0", targetName)
	}
	t.Logf("FindClient(Name=%q) returned %d results", targetName, len(results))

	// Verify the first result has the expected structure.
	r := results[0]
	if r.Client.ID <= 0 {
		t.Errorf("result.Client.ID expected > 0, got %d", r.Client.ID)
	}
	if r.Client.Name == "" {
		t.Error("result.Client.Name expected non-empty")
	}
	t.Logf("Client: id=%d name=%q branches=%d contacts=%d", r.Client.ID, r.Client.Name, len(r.Branches), len(r.Contacts))
}

func TestE2E_FindClient_ByText(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	clients, err := api.ListClients(ctx)
	if err != nil || len(clients) == 0 {
		t.Skip("no clients available")
	}
	// Use first 3 chars of the first client name as text query.
	name := clients[0].Name
	if len(name) < 3 {
		t.Skip("client name too short for text search")
	}
	text := name[:3]

	results, err := svc.FindClient(ctx, find.FindClientQuery{
		Text:  text,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindClient(Text=%q): %v", text, err)
	}
	t.Logf("FindClient(Text=%q) returned %d results", text, len(results))
}

// --- FindUser ---

func TestE2E_FindUser_ByName(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	users, err := api.ListUsers(ctx)
	if err != nil || len(users) == 0 {
		t.Skip("no users available")
	}
	// Find the first user with a non-empty name.
	targetName := ""
	for _, u := range users {
		if u.DisplayName() != "" {
			targetName = u.DisplayName()
			break
		}
	}
	if targetName == "" {
		t.Skip("no users with non-empty name found")
	}

	results, err := svc.FindUser(ctx, find.FindUserQuery{
		Name:  targetName,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindUser(Name=%q): %v", targetName, err)
	}
	if len(results) == 0 {
		t.Errorf("FindUser(Name=%q): expected >= 1 result, got 0", targetName)
	}
	t.Logf("FindUser(Name=%q) returned %d results", targetName, len(results))

	r := results[0]
	if r.User.ID <= 0 {
		t.Errorf("result.User.ID expected > 0, got %d", r.User.ID)
	}
}

// --- FindProject ---

func TestE2E_FindProject_ByID(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 1)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	targetID := pr.Items[0].ID

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		ID:    targetID,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(ID=%d): %v", targetID, err)
	}
	if len(results) != 1 {
		t.Errorf("FindProject(ID=%d): expected exactly 1 result, got %d", targetID, len(results))
	}
	t.Logf("FindProject(ID=%d): project=%q client resolved=%v", targetID, results[0].Project.Name, results[0].Client != nil)
}

func TestE2E_FindProject_ByClientName(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	cr, err := api.ListClientsPage(ctx, 1, 1)
	if err != nil || len(cr.Items) == 0 {
		t.Skip("no clients available")
	}
	clientName := cr.Items[0].Name

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		ClientName: clientName,
		Limit:      10,
		Opts:       e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(ClientName=%q): %v", clientName, err)
	}
	t.Logf("FindProject(ClientName=%q) returned %d results", clientName, len(results))
}

// --- FindEstimate ---

func TestE2E_FindEstimate_ByProjectID(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 1)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	targetID := pr.Items[0].ID

	results, err := svc.FindEstimate(ctx, find.FindEstimateQuery{
		ProjectID: targetID,
		Limit:     5,
		Opts:      e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindEstimate(ProjectID=%d): %v", targetID, err)
	}
	t.Logf("FindEstimate(ProjectID=%d) returned %d results", targetID, len(results))
}

func TestE2E_FindProject_WithEstimate(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 1)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	targetName := pr.Items[0].Name

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		Name:  targetName,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(Name=%q): %v", targetName, err)
	}
	if len(results) == 0 {
		t.Skipf("FindProject(Name=%q): no results", targetName)
	}
	r := results[0]
	if r.Estimate != nil {
		t.Logf("Project %q enriched with estimate: id=%d title=%q", r.Project.Name, r.Estimate.ID, r.Estimate.Title)
	} else {
		t.Logf("Project %q has no estimate enrichment", r.Project.Name)
	}
}

// Note: FindInvoice E2E tests are omitted from the find layer because this account
// has 11,000+ invoices. Any invoice lookup (even by ClientID search) triggers full
// pagination and takes 2+ minutes. The invoices endpoint is already verified at the
// boardapi layer in internal/boardapi/e2e_test.go (TestE2E_Invoices_List).
