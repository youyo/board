//go:build e2e

// E2E tests for the service/find package using real BOARD API data.
// These tests verify cross-resource resolution (Client + Branches + Contacts, Project + Client, etc.)
// Estimated API calls per full run: ~20 (0.7% of the 3000/day rate limit).
//
// Usage:
//
//	BOARD_API_KEY=<key> BOARD_API_TOKEN=<token> go test -tags e2e -v -count=1 ./internal/service/find/ -run TestE2E

package find_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// idSet returns a sorted slice of IDs extracted from items using getID.
func idSet[T any](items []T, getID func(T) int) []int {
	out := make([]int, 0, len(items))
	for _, it := range items {
		out = append(out, getID(it))
	}
	sort.Ints(out)
	return out
}

// intSetEqual returns true if a and b contain the same sorted int values.
func intSetEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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
	// Integrity check: all branches and contacts should reference the parent client.
	for i, b := range r.Branches {
		if b.ClientID != 0 && b.ClientID != r.Client.ID {
			t.Errorf("Branches[%d].ClientID=%d want=%d", i, b.ClientID, r.Client.ID)
		}
	}
	for i, c := range r.Contacts {
		if c.ClientID != 0 && c.ClientID != r.Client.ID {
			t.Errorf("Contacts[%d].ClientID=%d want=%d", i, c.ClientID, r.Client.ID)
		}
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

	// Integrity check: all branches and contacts should reference their parent client.
	for _, r := range results {
		for i, b := range r.Branches {
			if b.ClientID != 0 && b.ClientID != r.Client.ID {
				t.Errorf("Text=%q result client=%d Branches[%d].ClientID=%d", text, r.Client.ID, i, b.ClientID)
			}
		}
		for i, c := range r.Contacts {
			if c.ClientID != 0 && c.ClientID != r.Client.ID {
				t.Errorf("Text=%q result client=%d Contacts[%d].ClientID=%d", text, r.Client.ID, i, c.ClientID)
			}
		}
	}
}

func TestE2E_FindClient_StrictEnrichment(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1. Discover a client with data.
	page, err := api.ListClientsPage(ctx, 1, 1)
	if err != nil || len(page.Items) == 0 {
		t.Skip("no clients available")
	}
	targetID := page.Items[0].ID

	// 2. FindClient(ID) → ClientResult.
	results, err := svc.FindClient(ctx, find.FindClientQuery{
		ID:   targetID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindClient(ID=%d): %v", targetID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindClient(ID=%d): expected 1 result, got %d", targetID, len(results))
	}
	r := results[0]

	// 3. Client enrichment check.
	if r.Client.ID != targetID {
		t.Errorf("result.Client.ID: got=%d want=%d", r.Client.ID, targetID)
	}

	// 4. Independent raw fetch for Branches; compare count + ID set.
	branchesRaw, err := api.SearchClientBranchesRaw(ctx, boardapi.ClientBranchSearchParams{ClientID: targetID})
	if err != nil {
		t.Fatalf("SearchClientBranchesRaw(ClientID=%d): %v", targetID, err)
	}
	var independentBranches []boardapi.ClientBranchEntity
	if err := json.Unmarshal(branchesRaw, &independentBranches); err != nil {
		t.Fatalf("unmarshal branches: %v", err)
	}
	if len(r.Branches) != len(independentBranches) {
		t.Errorf("Branches count mismatch: FindClient=%d independent=%d", len(r.Branches), len(independentBranches))
	}
	expectedBranchIDs := idSet(independentBranches, func(b boardapi.ClientBranchEntity) int { return b.ID })
	actualBranchIDs := idSet(r.Branches, func(b boardapi.ClientBranchEntity) int { return b.ID })
	if !intSetEqual(expectedBranchIDs, actualBranchIDs) {
		t.Errorf("Branches ID set mismatch:\n  want=%v\n  got =%v", expectedBranchIDs, actualBranchIDs)
	}
	// All branches should reference the parent client.
	for i, b := range r.Branches {
		if b.ClientID != 0 && b.ClientID != targetID {
			t.Errorf("Branches[%d].ClientID=%d want=%d", i, b.ClientID, targetID)
		}
	}

	// 5. Independent raw fetch for Contacts; compare count + ID set.
	contactsRaw, err := api.SearchContactsRaw(ctx, boardapi.ContactSearchParams{ClientID: targetID})
	if err != nil {
		t.Fatalf("SearchContactsRaw(ClientID=%d): %v", targetID, err)
	}
	var independentContacts []boardapi.ContactEntity
	if err := json.Unmarshal(contactsRaw, &independentContacts); err != nil {
		t.Fatalf("unmarshal contacts: %v", err)
	}
	if len(r.Contacts) != len(independentContacts) {
		t.Errorf("Contacts count mismatch: FindClient=%d independent=%d", len(r.Contacts), len(independentContacts))
	}
	expectedContactIDs := idSet(independentContacts, func(c boardapi.ContactEntity) int { return c.ID })
	actualContactIDs := idSet(r.Contacts, func(c boardapi.ContactEntity) int { return c.ID })
	if !intSetEqual(expectedContactIDs, actualContactIDs) {
		t.Errorf("Contacts ID set mismatch:\n  want=%v\n  got =%v", expectedContactIDs, actualContactIDs)
	}
	for i, c := range r.Contacts {
		if c.ClientID != 0 && c.ClientID != targetID {
			t.Errorf("Contacts[%d].ClientID=%d want=%d", i, c.ClientID, targetID)
		}
	}

	t.Logf("FindClient(ID=%d) enrichment: branches=%d contacts=%d (both matched independent API)",
		targetID, len(r.Branches), len(r.Contacts))
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
		t.Logf("Project %q enriched with estimate: id=%d", r.Project.Name, r.Estimate.ID)
	} else {
		t.Logf("Project %q has no estimate enrichment", r.Project.Name)
	}
}

// Note: FindInvoice E2E tests are omitted from the find layer because this account
// has 11,000+ invoices. Any invoice lookup (even by ClientID search) triggers full
// pagination and takes 2+ minutes. The invoices endpoint is already verified at the
// boardapi layer in internal/boardapi/e2e_test.go (TestE2E_Invoices_List).
