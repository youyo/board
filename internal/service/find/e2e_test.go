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
	"strings"
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
	// M39: branch は ClientID() accessor 経由（nested Client）。
	// M40: contact も ClientID() accessor 経由（nested Client）。
	for i, b := range r.Branches {
		if b.ClientID() != 0 && b.ClientID() != r.Client.ID {
			t.Errorf("Branches[%d].ClientID=%d want=%d", i, b.ClientID(), r.Client.ID)
		}
	}
	for i, c := range r.Contacts {
		if c.ClientID() != 0 && c.ClientID() != r.Client.ID {
			t.Errorf("Contacts[%d].ClientID=%d want=%d", i, c.ClientID(), r.Client.ID)
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
	// M39: branch は ClientID() accessor 経由。M40: contact も ClientID() accessor 経由。
	for _, r := range results {
		for i, b := range r.Branches {
			if b.ClientID() != 0 && b.ClientID() != r.Client.ID {
				t.Errorf("Text=%q result client=%d Branches[%d].ClientID=%d", text, r.Client.ID, i, b.ClientID())
			}
		}
		for i, c := range r.Contacts {
			if c.ClientID() != 0 && c.ClientID() != r.Client.ID {
				t.Errorf("Text=%q result client=%d Contacts[%d].ClientID=%d", text, r.Client.ID, i, c.ClientID())
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
	// M39: branch は ClientID() accessor 経由。
	for i, b := range r.Branches {
		if b.ClientID() != 0 && b.ClientID() != targetID {
			t.Errorf("Branches[%d].ClientID=%d want=%d", i, b.ClientID(), targetID)
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
		if c.ClientID() != 0 && c.ClientID() != targetID {
			t.Errorf("Contacts[%d].ClientID=%d want=%d", i, c.ClientID(), targetID)
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
	t.Logf("FindUser(Name=%q) returned %d results", targetName, len(results))
	// BOARD API が name filter を無視して 0 件を返す場合もあるため、空結果は data-dependent skip。
	if len(results) == 0 {
		t.Skipf("FindUser(Name=%q): no results (BOARD API may ignore name filter)", targetName)
	}

	r := results[0]
	if r.User.ID <= 0 {
		t.Errorf("result.User.ID expected > 0, got %d", r.User.ID)
	}
}

// TestE2E_FindUser_ByID_Strict は ID モードで FindUser が正しく User を返し、
// DisplayName フォールバック経路が動作していることを検証する。
func TestE2E_FindUser_ByID_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	users, err := api.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) == 0 {
		t.Skip("no users available")
	}
	targetUser := users[0]

	results, err := svc.FindUser(ctx, find.FindUserQuery{
		ID:   targetUser.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindUser(ID=%d): %v", targetUser.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindUser(ID=%d): expected 1 result, got %d", targetUser.ID, len(results))
	}
	r := results[0]

	// ID 一致確認
	if r.User.ID != targetUser.ID {
		t.Errorf("r.User.ID: got=%d want=%d", r.User.ID, targetUser.ID)
	}

	// DisplayName フォールバック動作ログ（Name が空でも LastName/FirstName から生成される可能性あり）
	dn := r.User.DisplayName()
	t.Logf("User(id=%d): Name=%q LastName=%q FirstName=%q Email=%q → DisplayName()=%q",
		r.User.ID, r.User.Name, r.User.LastName, r.User.FirstName, r.User.Email, dn)
	// ログのみ（空である可能性もあるので assert しない）
}

// TestE2E_FindUser_ByName_StrictAddon は既存 TestE2E_FindUser_ByName への追補テスト。
// DisplayName 動作確認と ID 整合性を検証する。
func TestE2E_FindUser_ByName_StrictAddon(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	users, err := api.ListUsers(ctx)
	if err != nil || len(users) == 0 {
		t.Skip("no users available")
	}
	// 非空 DisplayName を持つ最初のユーザーを使用
	var targetUser *boardapi.UserEntity
	for i := range users {
		if users[i].DisplayName() != "" {
			targetUser = &users[i]
			break
		}
	}
	if targetUser == nil {
		t.Skip("no users with non-empty DisplayName found")
	}

	results, err := svc.FindUser(ctx, find.FindUserQuery{
		Name:  targetUser.DisplayName(),
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindUser(Name=%q): %v", targetUser.DisplayName(), err)
	}
	t.Logf("FindUser(Name=%q) returned %d results", targetUser.DisplayName(), len(results))
	// BOARD API が name filter を無視して 0 件を返す場合もある
	if len(results) == 0 {
		t.Skipf("FindUser(Name=%q): no results (BOARD API may ignore name filter)", targetUser.DisplayName())
	}

	// 各結果で ID > 0 と DisplayName の整合性を確認
	for i, r := range results {
		if r.User.ID <= 0 {
			t.Errorf("results[%d].User.ID expected > 0, got %d", i, r.User.ID)
		}
		dn := r.User.DisplayName()
		t.Logf("results[%d]: User(id=%d, DisplayName=%q)", i, r.User.ID, dn)
	}
}

// TestE2E_FindGroup_ByID_Strict は ID モードで FindGroup が正しく Group を返すことを検証する。
// 当該アカウントは groups 0 件と M07 で記録されているため、データなしの場合はスキップ。
func TestE2E_FindGroup_ByID_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	groups, err := api.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) == 0 {
		t.Skip("no groups; pending re-verification (group data not yet populated in this BOARD account)")
	}
	targetGroup := groups[0]

	results, err := svc.FindGroup(ctx, find.FindGroupQuery{
		ID:   targetGroup.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindGroup(ID=%d): %v", targetGroup.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindGroup(ID=%d): expected 1 result, got %d", targetGroup.ID, len(results))
	}
	r := results[0]

	// ID 一致確認
	if r.Group.ID != targetGroup.ID {
		t.Errorf("r.Group.ID: got=%d want=%d", r.Group.ID, targetGroup.ID)
	}
	// Name 一致確認
	if r.Group.Name != targetGroup.Name {
		t.Errorf("r.Group.Name: got=%q want=%q", r.Group.Name, targetGroup.Name)
	}

	t.Logf("FindGroup(ID=%d): name=%q", r.Group.ID, r.Group.Name)
}

// TestE2E_FindGroup_ByName は Name モードで FindGroup が動作することを検証する。
// 当該アカウントは groups 0 件と M07 で記録されているため、データなしの場合はスキップ。
func TestE2E_FindGroup_ByName(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	groups, err := api.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) == 0 {
		t.Skip("no groups; pending re-verification (group data not yet populated in this BOARD account)")
	}
	var targetGroup *boardapi.GroupEntity
	for i := range groups {
		if groups[i].Name != "" {
			targetGroup = &groups[i]
			break
		}
	}
	if targetGroup == nil {
		t.Skip("no groups with non-empty name found")
	}

	results, err := svc.FindGroup(ctx, find.FindGroupQuery{
		Name:  targetGroup.Name,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindGroup(Name=%q): %v", targetGroup.Name, err)
	}
	if len(results) == 0 {
		t.Errorf("FindGroup(Name=%q): expected >= 1 result, got 0", targetGroup.Name)
	}

	// 各結果で ID > 0 を確認
	for i, r := range results {
		if r.Group.ID <= 0 {
			t.Errorf("results[%d].Group.ID expected > 0, got %d", i, r.Group.ID)
		}
		t.Logf("results[%d]: Group(id=%d, name=%q)", i, r.Group.ID, r.Group.Name)
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

// TestE2E_FindProject_StrictEnrichment_ByID はプロジェクト ID による検索で
// Client と Estimate の enrichment が欠損しないことを独立 API 呼び出しと突合して検証する。
func TestE2E_FindProject_StrictEnrichment_ByID(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 5 件取得して最初の非ゼロ ClientID を持つプロジェクトを選ぶ。
	pr, err := api.ListProjectsPage(ctx, 1, 5)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	var targetProject *boardapi.ProjectEntity
	for i := range pr.Items {
		// M44: ClientID 廃止、Client nested の ID で判定
		if pr.Items[i].Client != nil && pr.Items[i].Client.ID != 0 {
			targetProject = &pr.Items[i]
			break
		}
	}
	if targetProject == nil {
		t.Skip("no project with non-zero Client.ID found in first 5 projects")
	}

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		ID:   targetProject.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(ID=%d): %v", targetProject.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindProject(ID=%d): expected 1 result, got %d", targetProject.ID, len(results))
	}
	r := results[0]

	// Project ID 一致確認。
	if r.Project.ID != targetProject.ID {
		t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, targetProject.ID)
	}

	// Client enrichment: M44: Client nested の ID で判定
	targetClientID := 0
	if targetProject.Client != nil {
		targetClientID = targetProject.Client.ID
	}
	if r.Client == nil {
		t.Errorf("r.Client is nil for project with Client.ID=%d; enrichment missing", targetClientID)
	} else {
		if r.Client.ID != targetClientID {
			t.Errorf("r.Client.ID: got=%d want=%d", r.Client.ID, targetClientID)
		}
		// 独立 API で突合。
		independentClient, err := api.GetClient(ctx, targetClientID)
		if err != nil {
			t.Fatalf("GetClient(ID=%d): %v", targetClientID, err)
		}
		if r.Client.ID != independentClient.ID {
			t.Errorf("client ID mismatch: FindProject=%d independent=%d", r.Client.ID, independentClient.ID)
		}
		t.Logf("Client enrichment OK: id=%d name=%q", r.Client.ID, r.Client.Name)
	}

	// Estimate enrichment: GetProjectWithGroup で response_group=estimate を叩いて突合。
	pw, err := api.GetProjectWithGroup(ctx, targetProject.ID, "estimate")
	if err != nil {
		t.Fatalf("GetProjectWithGroup(ID=%d, estimate): %v", targetProject.ID, err)
	}
	if pw.Estimate != nil {
		if r.Estimate == nil {
			t.Errorf("r.Estimate is nil but project has estimate id=%d", pw.Estimate.ID)
		} else {
			if r.Estimate.ID != pw.Estimate.ID {
				t.Errorf("r.Estimate.ID: got=%d want=%d", r.Estimate.ID, pw.Estimate.ID)
			}
			// M35 準拠フィールドチェック。
			if r.Estimate.Total == "" {
				t.Errorf("r.Estimate.Total is empty (EstimateEntity.Total should be a string amount)")
			}
			if r.Estimate.Details == nil {
				t.Logf("r.Estimate.Details is nil (may be empty estimate with no line items)")
			}
			t.Logf("Estimate enrichment OK: id=%d total=%q", r.Estimate.ID, r.Estimate.Total)
		}
	} else {
		if r.Estimate != nil {
			t.Errorf("r.Estimate expected nil (project has no estimate in response_group), got id=%d", r.Estimate.ID)
		}
		t.Logf("Project has no estimate enrichment (GetProjectWithGroup returned nil Estimate)")
	}
}

// TestE2E_FindProject_ByName_Strict は Name モードで空でない結果が返り、
// 各 result の Client enrichment が整合していることを確認する。
func TestE2E_FindProject_ByName_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 1)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	targetName := pr.Items[0].Name
	if targetName == "" {
		t.Skip("first project has empty name")
	}

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		Name:  targetName,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(Name=%q): %v", targetName, err)
	}
	if len(results) == 0 {
		t.Skipf("FindProject(Name=%q): no results (BOARD API may ignore name filter)", targetName)
	}
	t.Logf("FindProject(Name=%q) returned %d results", targetName, len(results))

	// 各 result で Client enrichment の整合性を確認する。
	// M44: ClientID 廃止、Client nested の ID で判定
	for i, r := range results {
		pClientID := 0
		if r.Project.Client != nil {
			pClientID = r.Project.Client.ID
		}
		if pClientID != 0 && r.Client == nil {
			t.Errorf("results[%d]: project.Client.ID=%d but r.Client is nil (enrichment missing)",
				i, pClientID)
		}
		if r.Client != nil && r.Client.ID != pClientID {
			t.Errorf("results[%d]: r.Client.ID=%d != r.Project.Client.ID=%d",
				i, r.Client.ID, pClientID)
		}
		t.Logf("results[%d]: project.ID=%d name=%q client_id=%d client_resolved=%v estimate=%v",
			i, r.Project.ID, r.Project.Name, pClientID, r.Client != nil, r.Estimate != nil)
	}
}

// TestE2E_FindProject_ByClientName_Strict は ClientName モードで Client filter が
// 機能しているかを独立 API 呼び出しと突合して検証する。
func TestE2E_FindProject_ByClientName_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	cr, err := api.ListClientsPage(ctx, 1, 1)
	if err != nil || len(cr.Items) == 0 {
		t.Skip("no clients available")
	}
	clientName := cr.Items[0].Name
	if clientName == "" {
		t.Skip("first client has empty name")
	}

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		ClientName: clientName,
		Limit:      3,
		Opts:       e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(ClientName=%q): %v", clientName, err)
	}
	t.Logf("FindProject(ClientName=%q) returned %d results", clientName, len(results))

	// 独立 API で一致する client ID 集合を取得する。
	// Note: BOARD API は name filter を無視するため searchClients は全件返す可能性がある。
	independentClients, err := api.SearchClients(ctx, boardapi.ClientSearchParams{Name: clientName})
	if err != nil {
		t.Fatalf("SearchClients(Name=%q): %v", clientName, err)
	}
	if len(independentClients) == 0 {
		t.Skipf("SearchClients(Name=%q): returned 0 results; data-dependent skip", clientName)
	}

	expectedClientIDs := make(map[int]bool, len(independentClients))
	for _, c := range independentClients {
		expectedClientIDs[c.ID] = true
	}
	t.Logf("Independent SearchClients returned %d clients (BOARD API may ignore name filter)", len(independentClients))

	// 各 result の Client enrichment 整合性確認。
	// M44: ClientID 廃止、Client nested の ID で判定
	for i, r := range results {
		pClientID := 0
		if r.Project.Client != nil {
			pClientID = r.Project.Client.ID
		}
		if pClientID != 0 && r.Client == nil {
			t.Errorf("results[%d]: project.Client.ID=%d but r.Client is nil (enrichment missing)",
				i, pClientID)
		}
		if r.Client != nil && r.Client.ID != pClientID {
			t.Errorf("results[%d]: r.Client.ID=%d != r.Project.Client.ID=%d",
				i, r.Client.ID, pClientID)
		}
		t.Logf("results[%d]: project.ID=%d client_id=%d in_expected_set=%v",
			i, r.Project.ID, pClientID, expectedClientIDs[pClientID])
	}
}

// TestE2E_FindProject_ByText_Strict は Text モードで全結果が prefix を含むことを確認する。
func TestE2E_FindProject_ByText_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 1)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}
	name := pr.Items[0].Name
	if len([]rune(name)) < 3 {
		t.Skip("first project name too short for text search (need >= 3 runes)")
	}
	// 先頭3文字をテキストクエリとして使用。
	prefix := string([]rune(name)[:3])

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		Text:  prefix,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(Text=%q): %v", prefix, err)
	}
	t.Logf("FindProject(Text=%q) returned %d results", prefix, len(results))

	// find 層の local filter（containsText は Name/ManagementNo/InHouseMemo を対象）で絞られているので、
	// M44: Code/Memo 廃止、ManagementNo/InHouseMemo で代替。
	for i, r := range results {
		p := r.Project
		mgmtNo := ""
		if p.ManagementNo != nil {
			mgmtNo = *p.ManagementNo
		}
		memo := ""
		if p.InHouseMemo != nil {
			memo = *p.InHouseMemo
		}
		containsInAny := strings.Contains(p.Name, prefix) ||
			strings.Contains(mgmtNo, prefix) ||
			strings.Contains(memo, prefix)
		if !containsInAny {
			t.Errorf("results[%d]: project(id=%d, name=%q, management_no=%q, in_house_memo=%q) does not contain prefix %q",
				i, p.ID, p.Name, mgmtNo, memo, prefix)
		}
	}
}

// TestE2E_FindProject_ByStatus_Strict は Status モードで全結果のステータスが一致することを確認する。
func TestE2E_FindProject_ByStatus_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	pr, err := api.ListProjectsPage(ctx, 1, 5)
	if err != nil || len(pr.Items) == 0 {
		t.Skip("no projects available")
	}

	// M44: Status フィールド廃止。OrderStatusName で discovery する。
	statusCount := make(map[string]int)
	for _, p := range pr.Items {
		if p.OrderStatusName != "" {
			statusCount[p.OrderStatusName]++
		}
	}
	if len(statusCount) == 0 {
		t.Skip("no project with non-empty order_status_name found in first 5 projects")
	}
	var targetStatus string
	var maxCount int
	for s, c := range statusCount {
		if c > maxCount {
			maxCount = c
			targetStatus = s
		}
	}

	results, err := svc.FindProject(ctx, find.FindProjectQuery{
		Status: targetStatus,
		Limit:  3,
		Opts:   e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindProject(Status=%q): %v", targetStatus, err)
	}
	t.Logf("FindProject(Status=%q) returned %d results", targetStatus, len(results))

	// M44: OrderStatusName / DeliveryStatusName のいずれかが targetStatus と一致することを確認
	for i, r := range results {
		if r.Project.OrderStatusName != targetStatus && r.Project.DeliveryStatusName != targetStatus {
			t.Errorf("results[%d]: project.OrderStatusName=%q / DeliveryStatusName=%q want=%q",
				i, r.Project.OrderStatusName, r.Project.DeliveryStatusName, targetStatus)
		}
	}
}

// --- FindInvoice (M32) ---

// Note: FindInvoice E2E tests use only the ID mode because this account has 11,000+ invoices.
// ClientName / ProjectName / Text / Status modes trigger full pagination and take 2+ minutes.
// The invoices endpoint is already verified at the boardapi layer in internal/boardapi/e2e_test.go.
// ID mode uses ListInvoicesPage(ctx, 1, 1) to fetch just one invoice, then resolves by ID.

// TestE2E_FindInvoice_ByID_Strict は ID モードで FindInvoice が正しく Invoice を返し、
// Client/Project の enrichment が整合していることを検証する。
// per_page=1 で先頭 1 件のみ取得し、全件 fetch を避ける軽量設計。
func TestE2E_FindInvoice_ByID_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// per_page=1 で先頭 1 件だけ取得（全件 pagination を避ける）
	page, err := api.ListInvoicesPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListInvoicesPage(1, 1): %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no invoices available")
	}
	inv := page.Items[0]

	results, err := svc.FindInvoice(ctx, find.FindInvoiceQuery{
		ID:   inv.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindInvoice(ID=%d): %v", inv.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindInvoice(ID=%d): expected 1 result, got %d", inv.ID, len(results))
	}
	r := results[0]

	// Invoice ID 一致確認
	if r.Invoice.ID != inv.ID {
		t.Errorf("r.Invoice.ID: got=%d want=%d", r.Invoice.ID, inv.ID)
	}

	// Client enrichment: ClientID 非ゼロなら Client は nil 不可
	if inv.ClientID != 0 {
		if r.Client == nil {
			t.Errorf("r.Client is nil but Invoice.ClientID=%d (enrichment missing)", inv.ClientID)
		} else if r.Client.ID != inv.ClientID {
			t.Errorf("r.Client.ID: got=%d want=%d", r.Client.ID, inv.ClientID)
		} else {
			t.Logf("Client enrichment OK: id=%d", r.Client.ID)
		}
	} else {
		if r.Client != nil {
			t.Errorf("r.Client expected nil (Invoice.ClientID=0), got id=%d", r.Client.ID)
		}
	}

	// Project enrichment: ProjectID 非ゼロなら Project は nil 不可
	if inv.ProjectID != 0 {
		if r.Project == nil {
			t.Errorf("r.Project is nil but Invoice.ProjectID=%d (enrichment missing)", inv.ProjectID)
		} else if r.Project.ID != inv.ProjectID {
			t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, inv.ProjectID)
		} else {
			t.Logf("Project enrichment OK: id=%d", r.Project.ID)
		}
	} else {
		if r.Project != nil {
			t.Errorf("r.Project expected nil (Invoice.ProjectID=0), got id=%d", r.Project.ID)
		}
	}

	t.Logf("FindInvoice(ID=%d): clientID=%d client_resolved=%v projectID=%d project_resolved=%v",
		inv.ID, inv.ClientID, r.Client != nil, inv.ProjectID, r.Project != nil)
}

// TestE2E_FindInvoice_ByClientName は ClientName モード。
// 11,000+ invoices のため cache-warm 必須。スキップ。
func TestE2E_FindInvoice_ByClientName(t *testing.T) {
	t.Skip("cache-warm required; 11000+ invoices. Run after cache warm: go test -tags e2e -timeout 30m -run TestE2E_FindInvoice_ByClientName ./internal/service/find/")
}

// TestE2E_FindInvoice_ByProjectName は ProjectName モード。
// 11,000+ invoices のため cache-warm 必須。スキップ。
func TestE2E_FindInvoice_ByProjectName(t *testing.T) {
	t.Skip("cache-warm required; 11000+ invoices. Run after cache warm: go test -tags e2e -timeout 30m -run TestE2E_FindInvoice_ByProjectName ./internal/service/find/")
}

// TestE2E_FindInvoice_ByText は Text モード。
// 11,000+ invoices のため cache-warm 必須。スキップ。
func TestE2E_FindInvoice_ByText(t *testing.T) {
	t.Skip("cache-warm required; 11000+ invoices. Run after cache warm: go test -tags e2e -timeout 30m -run TestE2E_FindInvoice_ByText ./internal/service/find/")
}

// TestE2E_FindInvoice_ByStatus は Status モード。
// 11,000+ invoices のため cache-warm 必須。スキップ。
func TestE2E_FindInvoice_ByStatus(t *testing.T) {
	t.Skip("cache-warm required; 11000+ invoices. Run after cache warm: go test -tags e2e -timeout 30m -run TestE2E_FindInvoice_ByStatus ./internal/service/find/")
}

// --- findProjectWithDocType helper ---

// findProjectWithDocType は先頭 topN 件のプロジェクトを走査し、
// 指定された docType ("order"/"delivery"/"receipt") のドキュメントを持つ
// 最初のプロジェクトの (projectID, documentID) を返す。
// 見つからなければ (0, 0) を返す（t.Skip は呼ばない）。
//
// delivery/receipt は API レスポンスで複数形配列 ("deliveries"/"receipts") として返るため、
// raw JSON を直接 probe struct で解析する。
// order は単数形オブジェクト ("order") として返る。
func findProjectWithDocType(t *testing.T, apiClient *boardapi.Client, docType string, topN int) (projectID, documentID int) {
	t.Helper()
	ctx := context.Background()

	// topN 件を 1 ページで取得（全ページ走査を避ける）
	page, err := apiClient.ListProjectsPage(ctx, 1, topN)
	if err != nil || len(page.Items) == 0 {
		t.Logf("findProjectWithDocType: ListProjectsPage failed or empty: %v", err)
		return 0, 0
	}

	for _, p := range page.Items {
		if p.ID <= 0 {
			continue
		}
		raw, err := apiClient.GetProjectWithGroupRaw(ctx, p.ID, docType)
		if err != nil {
			t.Logf("findProjectWithDocType: GetProjectWithGroupRaw(%d, %s): %v (continuing)", p.ID, docType, err)
			continue
		}

		// probe struct: order は単数形、delivery/receipt は複数形配列
		var probe struct {
			Order *struct {
				ID int `json:"id"`
			} `json:"order"`
			Deliveries []struct {
				ID int `json:"id"`
			} `json:"deliveries"`
			Receipts []struct {
				ID int `json:"id"`
			} `json:"receipts"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Logf("findProjectWithDocType: unmarshal project %d: %v (continuing)", p.ID, err)
			continue
		}

		var docID int
		switch docType {
		case "order":
			if probe.Order != nil && probe.Order.ID > 0 {
				docID = probe.Order.ID
			}
		case "delivery":
			if len(probe.Deliveries) > 0 && probe.Deliveries[0].ID > 0 {
				docID = probe.Deliveries[0].ID
			}
		case "receipt":
			if len(probe.Receipts) > 0 && probe.Receipts[0].ID > 0 {
				docID = probe.Receipts[0].ID
			}
		}

		if docID > 0 {
			// M44: ClientID 廃止、Client nested の ID で表示
			clientIDForLog := 0
			if p.Client != nil {
				clientIDForLog = p.Client.ID
			}
			t.Logf("findProjectWithDocType: found docType=%s projectID=%d documentID=%d clientID=%d",
				docType, p.ID, docID, clientIDForLog)
			return p.ID, docID
		}
	}

	return 0, 0
}

// --- FindOrder (M27) ---

// TestE2E_FindOrder_ByProjectID_Strict は ProjectID モードで
// FindOrder が正しく Order を返し、Client/Project enrichment が整合していることを検証する。
func TestE2E_FindOrder_ByProjectID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	// discovery: 先頭 50 件から order を持つプロジェクトを探す
	projectID, expectedDocID := findProjectWithDocType(t, apiClient, "order", 50)
	if projectID == 0 || expectedDocID == 0 {
		t.Skip("no project with order found in top-50; pending re-verification")
	}

	// pre-fetch: projectID の ClientID を取得（enrichment 検証用）
	pr, err := apiClient.GetProjectWithGroup(ctx, projectID, "order")
	if err != nil {
		t.Fatalf("GetProjectWithGroup(%d, order): %v", projectID, err)
	}

	results, err := svc.FindOrder(ctx, find.FindOrderQuery{
		ProjectID: projectID,
		Opts:      e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindOrder(ProjectID=%d): %v", projectID, err)
	}
	if len(results) == 0 {
		t.Fatalf("FindOrder(ProjectID=%d): expected >= 1 result, got 0", projectID)
	}
	r := results[0]

	// Order ID 一致確認
	if r.Order.ID != expectedDocID {
		t.Errorf("r.Order.ID: got=%d want=%d", r.Order.ID, expectedDocID)
	}

	// 独立 raw fetch で厳格フィールド突合
	raw, err := apiClient.GetOrderRaw(ctx, expectedDocID)
	if err != nil {
		t.Fatalf("GetOrderRaw(%d): %v", expectedDocID, err)
	}
	if diff := strictFieldDiff(t, raw, &boardapi.OrderEntity{}); len(diff) > 0 {
		t.Errorf("OrderEntity unmapped fields: %v", diff)
	}

	// Client enrichment — M44: ClientID 廃止、Client nested の ID で判定
	prClientID := 0
	if pr.Client != nil {
		prClientID = pr.Client.ID
	}
	if prClientID == 0 {
		if r.Client != nil {
			t.Errorf("r.Client expected nil (project.Client.ID=0), got id=%d", r.Client.ID)
		}
		t.Logf("Client enrichment: project.Client.ID=0, r.Client=nil (expected)")
	} else {
		if r.Client == nil {
			t.Errorf("r.Client is nil but project.Client.ID=%d (enrichment missing)", prClientID)
		} else if r.Client.ID != prClientID {
			t.Errorf("r.Client.ID: got=%d want=%d", r.Client.ID, prClientID)
		} else {
			t.Logf("Client enrichment OK: id=%d", r.Client.ID)
		}
	}

	// Project enrichment
	if r.Project == nil {
		t.Errorf("r.Project is nil (enrichment missing)")
	} else if r.Project.ID != projectID {
		t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, projectID)
	} else {
		t.Logf("Project enrichment OK: id=%d name=%q", r.Project.ID, r.Project.Name)
	}

	t.Logf("FindOrder(ProjectID=%d): order.ID=%d total=%s client_resolved=%v project_resolved=%v",
		projectID, r.Order.ID, r.Order.Total, r.Client != nil, r.Project != nil)
}

// TestE2E_FindOrder_ByID_Strict は ID モードで FindOrder が直接 Order を返し、
// Client/Project が nil（ID モード仕様: 特定不可）であることを検証する。
func TestE2E_FindOrder_ByID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	_, docID := findProjectWithDocType(t, apiClient, "order", 50)
	if docID == 0 {
		t.Skip("no order found in top-50; pending re-verification")
	}

	results, err := svc.FindOrder(ctx, find.FindOrderQuery{
		ID:   docID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindOrder(ID=%d): %v", docID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindOrder(ID=%d): expected 1 result, got %d", docID, len(results))
	}
	r := results[0]

	if r.Order.ID != docID {
		t.Errorf("r.Order.ID: got=%d want=%d", r.Order.ID, docID)
	}
	// ID モード仕様: Client/Project は特定不可なので nil
	if r.Client != nil {
		t.Errorf("r.Client expected nil in ID mode, got id=%d", r.Client.ID)
	}
	if r.Project != nil {
		t.Errorf("r.Project expected nil in ID mode, got id=%d", r.Project.ID)
	}

	t.Logf("FindOrder(ID=%d): order.ID=%d total=%s client=nil project=nil (ID mode OK)",
		docID, r.Order.ID, r.Order.Total)
}

// TestE2E_FindOrder_ByClientName_Strict は ClientName モードでエラーなしを確認する。
// 当該アカウントは projects.ClientID=0 が多いため、結果ゼロ可。
//
// NOTE: BOARD API は name filter を無視して全件（299件+）を返す。
// ClientName モードは clients 全件 → projects 全件 → orders 個別 fetch の連鎖となり、
// キャッシュウォームアップなし（初回実行）ではタイムアウトする。
// このテストはキャッシュ済み環境でのみ実行すること。
func TestE2E_FindOrder_ByClientName_Strict(t *testing.T) {
	t.Skip("ClientName mode requires pre-warmed cache; skipping to avoid full-fetch timeout (~10min). " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindOrder_ByClientName_Strict ./internal/service/find/")
}

// TestE2E_FindOrder_ByProjectName_Strict は ProjectName モードで
// 返却された各 result の enrichment 整合性を確認する。
//
// NOTE: BOARD API が name filter を無視して projects 全件を返すため、
// 初回キャッシュなし実行では全 project の order を個別 fetch する連鎖が発生する。
// キャッシュウォームアップ済み環境でのみ実行すること。
func TestE2E_FindOrder_ByProjectName_Strict(t *testing.T) {
	t.Skip("ProjectName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindOrder_ByProjectName_Strict ./internal/service/find/")
}

// --- FindDelivery (M28) ---

// TestE2E_FindDelivery_ByProjectID_Strict は ProjectID モードで
// FindDelivery が正しく Delivery を返し、Client/Project enrichment が整合していることを検証する。
// M28: ProjectEntity.Delivery（単数形）→ Deliveries（複数形配列）への fix 後に PASS を期待。
func TestE2E_FindDelivery_ByProjectID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	// discovery: 先頭 50 件から delivery を持つプロジェクトを探す
	projectID, expectedDocID := findProjectWithDocType(t, apiClient, "delivery", 50)
	if projectID == 0 || expectedDocID == 0 {
		t.Skip("no project with delivery found in top-50; pending re-verification")
	}

	// pre-fetch: projectID の ClientID を取得（enrichment 検証用）
	pr, err := apiClient.GetProjectWithGroup(ctx, projectID, "delivery")
	if err != nil {
		t.Fatalf("GetProjectWithGroup(%d, delivery): %v", projectID, err)
	}

	results, err := svc.FindDelivery(ctx, find.FindDeliveryQuery{
		ProjectID: projectID,
		Opts:      e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindDelivery(ProjectID=%d): %v", projectID, err)
	}
	if len(results) == 0 {
		t.Fatalf("FindDelivery(ProjectID=%d): expected >= 1 result, got 0 (possible ProjectEntity.Deliveries mapping bug)", projectID)
	}
	r := results[0]

	// Delivery ID 一致確認
	if r.Delivery.ID != expectedDocID {
		t.Errorf("r.Delivery.ID: got=%d want=%d", r.Delivery.ID, expectedDocID)
	}

	// delivery_date フィールド確認（DeliveryEntity 固有）
	if r.Delivery.DeliveryDate == "" {
		t.Errorf("r.Delivery.DeliveryDate is empty (should be non-empty per e2e-artifacts)")
	}

	// 独立 raw fetch で厳格フィールド突合
	raw, err := apiClient.GetDeliveryRaw(ctx, expectedDocID)
	if err != nil {
		t.Fatalf("GetDeliveryRaw(%d): %v", expectedDocID, err)
	}
	if diff := strictFieldDiff(t, raw, &boardapi.DeliveryEntity{}); len(diff) > 0 {
		t.Errorf("DeliveryEntity unmapped fields: %v", diff)
	}

	// Client enrichment — M44: ClientID 廃止、Client nested の ID で判定
	prClientID := 0
	if pr.Client != nil {
		prClientID = pr.Client.ID
	}
	if prClientID == 0 {
		if r.Client != nil {
			t.Errorf("r.Client expected nil (project.Client.ID=0), got id=%d", r.Client.ID)
		}
		t.Logf("Client enrichment: project.Client.ID=0, r.Client=nil (expected)")
	} else {
		if r.Client == nil {
			t.Errorf("r.Client is nil but project.Client.ID=%d (enrichment missing)", prClientID)
		} else if r.Client.ID != prClientID {
			t.Errorf("r.Client.ID: got=%d want=%d", r.Client.ID, prClientID)
		} else {
			t.Logf("Client enrichment OK: id=%d", r.Client.ID)
		}
	}

	// Project enrichment
	if r.Project == nil {
		t.Errorf("r.Project is nil (enrichment missing)")
	} else if r.Project.ID != projectID {
		t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, projectID)
	} else {
		t.Logf("Project enrichment OK: id=%d name=%q", r.Project.ID, r.Project.Name)
	}

	t.Logf("FindDelivery(ProjectID=%d): delivery.ID=%d delivery_date=%q client_resolved=%v project_resolved=%v",
		projectID, r.Delivery.ID, r.Delivery.DeliveryDate, r.Client != nil, r.Project != nil)
}

// TestE2E_FindDelivery_ByID_Strict は ID モードで FindDelivery が直接 Delivery を返し、
// Client/Project が nil（ID モード仕様）であることを検証する。
func TestE2E_FindDelivery_ByID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	_, docID := findProjectWithDocType(t, apiClient, "delivery", 50)
	if docID == 0 {
		t.Skip("no delivery found in top-50; pending re-verification")
	}

	results, err := svc.FindDelivery(ctx, find.FindDeliveryQuery{
		ID:   docID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindDelivery(ID=%d): %v", docID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindDelivery(ID=%d): expected 1 result, got %d", docID, len(results))
	}
	r := results[0]

	if r.Delivery.ID != docID {
		t.Errorf("r.Delivery.ID: got=%d want=%d", r.Delivery.ID, docID)
	}
	if r.Delivery.DeliveryDate == "" {
		t.Errorf("r.Delivery.DeliveryDate is empty")
	}
	// ID モード仕様: Client/Project は特定不可なので nil
	if r.Client != nil {
		t.Errorf("r.Client expected nil in ID mode, got id=%d", r.Client.ID)
	}
	if r.Project != nil {
		t.Errorf("r.Project expected nil in ID mode, got id=%d", r.Project.ID)
	}

	t.Logf("FindDelivery(ID=%d): delivery.ID=%d delivery_date=%q client=nil project=nil (ID mode OK)",
		docID, r.Delivery.ID, r.Delivery.DeliveryDate)
}

// TestE2E_FindDelivery_ByClientName_Strict は ClientName モード。cache-warm SKIP。
func TestE2E_FindDelivery_ByClientName_Strict(t *testing.T) {
	t.Skip("ClientName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindDelivery_ByClientName_Strict ./internal/service/find/")
}

// TestE2E_FindDelivery_ByProjectName_Strict は ProjectName モード。cache-warm SKIP。
func TestE2E_FindDelivery_ByProjectName_Strict(t *testing.T) {
	t.Skip("ProjectName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindDelivery_ByProjectName_Strict ./internal/service/find/")
}

// --- FindReceipt (M29) ---

// TestE2E_FindReceipt_ByProjectID_Strict は ProjectID モードで
// FindReceipt が正しく Receipt を返し、Client/Project enrichment が整合していることを検証する。
// M29: ProjectEntity.Receipt（単数形）→ Receipts（複数形配列）への fix 後に PASS を期待。
func TestE2E_FindReceipt_ByProjectID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	// discovery: 先頭 50 件から receipt を持つプロジェクトを探す
	projectID, expectedDocID := findProjectWithDocType(t, apiClient, "receipt", 50)
	if projectID == 0 || expectedDocID == 0 {
		t.Skip("no project with receipt found in top-50; pending re-verification")
	}

	// pre-fetch: projectID の ClientID を取得（enrichment 検証用）
	pr, err := apiClient.GetProjectWithGroup(ctx, projectID, "receipt")
	if err != nil {
		t.Fatalf("GetProjectWithGroup(%d, receipt): %v", projectID, err)
	}

	results, err := svc.FindReceipt(ctx, find.FindReceiptQuery{
		ProjectID: projectID,
		Opts:      e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindReceipt(ProjectID=%d): %v", projectID, err)
	}
	if len(results) == 0 {
		t.Fatalf("FindReceipt(ProjectID=%d): expected >= 1 result, got 0 (possible ProjectEntity.Receipts mapping bug)", projectID)
	}
	r := results[0]

	// Receipt ID 一致確認
	if r.Receipt.ID != expectedDocID {
		t.Errorf("r.Receipt.ID: got=%d want=%d", r.Receipt.ID, expectedDocID)
	}

	// receipt_date フィールド確認（ReceiptEntity 固有）
	if r.Receipt.ReceiptDate == "" {
		t.Errorf("r.Receipt.ReceiptDate is empty (should be non-empty per e2e-artifacts)")
	}

	// 独立 raw fetch で厳格フィールド突合
	raw, err := apiClient.GetReceiptRaw(ctx, expectedDocID)
	if err != nil {
		t.Fatalf("GetReceiptRaw(%d): %v", expectedDocID, err)
	}
	if diff := strictFieldDiff(t, raw, &boardapi.ReceiptEntity{}); len(diff) > 0 {
		t.Errorf("ReceiptEntity unmapped fields: %v", diff)
	}

	// Client enrichment — M44: ClientID 廃止、Client nested の ID で判定
	prClientID := 0
	if pr.Client != nil {
		prClientID = pr.Client.ID
	}
	if prClientID == 0 {
		if r.Client != nil {
			t.Errorf("r.Client expected nil (project.Client.ID=0), got id=%d", r.Client.ID)
		}
		t.Logf("Client enrichment: project.Client.ID=0, r.Client=nil (expected)")
	} else {
		if r.Client == nil {
			t.Errorf("r.Client is nil but project.Client.ID=%d (enrichment missing)", prClientID)
		} else if r.Client.ID != prClientID {
			t.Errorf("r.Client.ID: got=%d want=%d", r.Client.ID, prClientID)
		} else {
			t.Logf("Client enrichment OK: id=%d", r.Client.ID)
		}
	}

	// Project enrichment
	if r.Project == nil {
		t.Errorf("r.Project is nil (enrichment missing)")
	} else if r.Project.ID != projectID {
		t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, projectID)
	} else {
		t.Logf("Project enrichment OK: id=%d name=%q", r.Project.ID, r.Project.Name)
	}

	t.Logf("FindReceipt(ProjectID=%d): receipt.ID=%d receipt_date=%q client_resolved=%v project_resolved=%v",
		projectID, r.Receipt.ID, r.Receipt.ReceiptDate, r.Client != nil, r.Project != nil)
}

// TestE2E_FindReceipt_ByID_Strict は ID モードで FindReceipt が直接 Receipt を返し、
// Client/Project が nil（ID モード仕様）であることを検証する。
func TestE2E_FindReceipt_ByID_Strict(t *testing.T) {
	svc, apiClient := newE2EFindService(t)
	ctx := context.Background()

	_, docID := findProjectWithDocType(t, apiClient, "receipt", 50)
	if docID == 0 {
		t.Skip("no receipt found in top-50; pending re-verification")
	}

	results, err := svc.FindReceipt(ctx, find.FindReceiptQuery{
		ID:   docID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindReceipt(ID=%d): %v", docID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindReceipt(ID=%d): expected 1 result, got %d", docID, len(results))
	}
	r := results[0]

	if r.Receipt.ID != docID {
		t.Errorf("r.Receipt.ID: got=%d want=%d", r.Receipt.ID, docID)
	}
	if r.Receipt.ReceiptDate == "" {
		t.Errorf("r.Receipt.ReceiptDate is empty")
	}
	// ID モード仕様: Client/Project は特定不可なので nil
	if r.Client != nil {
		t.Errorf("r.Client expected nil in ID mode, got id=%d", r.Client.ID)
	}
	if r.Project != nil {
		t.Errorf("r.Project expected nil in ID mode, got id=%d", r.Project.ID)
	}

	t.Logf("FindReceipt(ID=%d): receipt.ID=%d receipt_date=%q client=nil project=nil (ID mode OK)",
		docID, r.Receipt.ID, r.Receipt.ReceiptDate)
}

// TestE2E_FindReceipt_ByClientName_Strict は ClientName モード。cache-warm SKIP。
func TestE2E_FindReceipt_ByClientName_Strict(t *testing.T) {
	t.Skip("ClientName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindReceipt_ByClientName_Strict ./internal/service/find/")
}

// TestE2E_FindReceipt_ByProjectName_Strict は ProjectName モード。cache-warm SKIP。
func TestE2E_FindReceipt_ByProjectName_Strict(t *testing.T) {
	t.Skip("ProjectName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindReceipt_ByProjectName_Strict ./internal/service/find/")
}

// --- FindVendor (M30) ---

// TestE2E_FindVendor_StrictEnrichment は ID モードで FindVendor が正しく Vendor を返し、
// Branches/Contacts の enrichment が独立 API 呼び出しと件数・ID 集合で一致することを検証する。
//
// 当該アカウントは vendors 0 件のためデータ投入まで pending re-verification。
// データが存在する場合は M25 と同型の enrichment バグ（VendorID フィルタ in-memory 誤動作）を検出する可能性がある。
func TestE2E_FindVendor_StrictEnrichment(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1. vendor 一覧取得 → 0 件ならスキップ
	page, err := api.ListVendorsPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListVendorsPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no vendors; pending re-verification (vendor data not yet populated in this BOARD account)")
	}
	targetID := page.Items[0].ID

	// 2. FindVendor(ID) → VendorResult
	results, err := svc.FindVendor(ctx, find.FindVendorQuery{
		ID:   targetID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindVendor(ID=%d): %v", targetID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindVendor(ID=%d): expected 1 result, got %d", targetID, len(results))
	}
	r := results[0]

	// 3. Vendor ID 一致確認
	if r.Vendor.ID != targetID {
		t.Errorf("r.Vendor.ID: got=%d want=%d", r.Vendor.ID, targetID)
	}

	// 4. 独立 raw fetch で Branches 件数・ID 集合を突合
	branchesRaw, err := api.SearchVendorBranchesRaw(ctx, boardapi.VendorBranchSearchParams{VendorID: targetID})
	if err != nil {
		t.Fatalf("SearchVendorBranchesRaw(VendorID=%d): %v", targetID, err)
	}
	var independentBranches []boardapi.VendorBranchEntity
	if err := json.Unmarshal(branchesRaw, &independentBranches); err != nil {
		t.Fatalf("unmarshal vendor branches: %v", err)
	}
	// data-dependent: Search が 1+ 件で FindVendor が 0 件なら enrichment バグ
	if len(independentBranches) > 0 && len(r.Branches) == 0 {
		t.Errorf("Branches enrichment bug detected: independent API returned %d branches, FindVendor returned 0",
			len(independentBranches))
	}
	if len(r.Branches) != len(independentBranches) {
		t.Errorf("Branches count mismatch: FindVendor=%d independent=%d", len(r.Branches), len(independentBranches))
	}
	expectedBranchIDs := idSet(independentBranches, func(b boardapi.VendorBranchEntity) int { return b.ID })
	actualBranchIDs := idSet(r.Branches, func(b boardapi.VendorBranchEntity) int { return b.ID })
	if !intSetEqual(expectedBranchIDs, actualBranchIDs) {
		t.Errorf("Branches ID set mismatch:\n  want=%v\n  got =%v", expectedBranchIDs, actualBranchIDs)
	}
	// 全 branch が親 vendor を参照していること
	for i, b := range r.Branches {
		if vid := b.VendorID(); vid != 0 && vid != targetID {
			t.Errorf("Branches[%d].VendorID=%d want=%d", i, vid, targetID)
		}
	}

	// 5. 独立 raw fetch で Contacts 件数・ID 集合を突合
	contactsRaw, err := api.SearchVendorContactsRaw(ctx, boardapi.VendorContactSearchParams{VendorID: targetID})
	if err != nil {
		t.Fatalf("SearchVendorContactsRaw(VendorID=%d): %v", targetID, err)
	}
	var independentContacts []boardapi.VendorContactEntity
	if err := json.Unmarshal(contactsRaw, &independentContacts); err != nil {
		t.Fatalf("unmarshal vendor contacts: %v", err)
	}
	// data-dependent: Search が 1+ 件で FindVendor が 0 件なら enrichment バグ
	if len(independentContacts) > 0 && len(r.Contacts) == 0 {
		t.Errorf("Contacts enrichment bug detected: independent API returned %d contacts, FindVendor returned 0",
			len(independentContacts))
	}
	if len(r.Contacts) != len(independentContacts) {
		t.Errorf("Contacts count mismatch: FindVendor=%d independent=%d", len(r.Contacts), len(independentContacts))
	}
	expectedContactIDs := idSet(independentContacts, func(c boardapi.VendorContactEntity) int { return c.ID })
	actualContactIDs := idSet(r.Contacts, func(c boardapi.VendorContactEntity) int { return c.ID })
	if !intSetEqual(expectedContactIDs, actualContactIDs) {
		t.Errorf("Contacts ID set mismatch:\n  want=%v\n  got =%v", expectedContactIDs, actualContactIDs)
	}
	for i, c := range r.Contacts {
		if vid := c.VendorID(); vid != 0 && vid != targetID {
			t.Errorf("Contacts[%d].VendorID=%d want=%d", i, vid, targetID)
		}
	}

	t.Logf("FindVendor(ID=%d) enrichment: branches=%d contacts=%d (both matched independent API)",
		targetID, len(r.Branches), len(r.Contacts))
}

// TestE2E_FindVendor_ByName は Name モードで vendor データ待ちのため SKIP。
func TestE2E_FindVendor_ByName(t *testing.T) {
	t.Skip("vendors 0 件; pending re-verification (vendor data not yet populated in this BOARD account)")
}

// TestE2E_FindVendor_ByText は Text モードで vendor データ待ちのため SKIP。
func TestE2E_FindVendor_ByText(t *testing.T) {
	t.Skip("vendors 0 件; pending re-verification (vendor data not yet populated in this BOARD account)")
}

// --- FindPurchaseOrder (M30) ---

// TestE2E_FindPurchaseOrder_ByID_Strict は ID モードで FindPurchaseOrder が正しく PurchaseOrder を返し、
// Vendor enrichment が整合していることを検証する。
//
// 当該アカウントは purchase_orders 0 件のため pending re-verification。
func TestE2E_FindPurchaseOrder_ByID_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1 件取得 → 0 件ならスキップ
	page, err := api.ListPurchaseOrdersPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListPurchaseOrdersPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no purchase_orders; pending re-verification (purchase_order data not yet populated in this BOARD account)")
	}
	targetPO := page.Items[0]

	results, err := svc.FindPurchaseOrder(ctx, find.FindPurchaseOrderQuery{
		ID:   targetPO.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPurchaseOrder(ID=%d): %v", targetPO.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindPurchaseOrder(ID=%d): expected 1 result, got %d", targetPO.ID, len(results))
	}
	r := results[0]

	// PO ID 一致確認
	if r.PurchaseOrder.ID != targetPO.ID {
		t.Errorf("r.PurchaseOrder.ID: got=%d want=%d", r.PurchaseOrder.ID, targetPO.ID)
	}

	// Vendor enrichment: VendorID 非ゼロなら nil 不可
	if targetPO.VendorID != 0 {
		if r.Vendor == nil {
			t.Errorf("r.Vendor is nil but PurchaseOrder.VendorID=%d (enrichment missing)", targetPO.VendorID)
		} else if r.Vendor.ID != targetPO.VendorID {
			t.Errorf("r.Vendor.ID: got=%d want=%d", r.Vendor.ID, targetPO.VendorID)
		} else {
			t.Logf("Vendor enrichment OK: id=%d", r.Vendor.ID)
		}
	}

	// Project enrichment: ProjectID 非ゼロなら nil 不可
	if targetPO.ProjectID != 0 {
		if r.Project == nil {
			t.Errorf("r.Project is nil but PurchaseOrder.ProjectID=%d (enrichment missing)", targetPO.ProjectID)
		} else if r.Project.ID != targetPO.ProjectID {
			t.Errorf("r.Project.ID: got=%d want=%d", r.Project.ID, targetPO.ProjectID)
		} else {
			t.Logf("Project enrichment OK: id=%d", r.Project.ID)
		}
	}

	t.Logf("FindPurchaseOrder(ID=%d): title=%q status=%q vendor_id=%d project_id=%d vendor_resolved=%v project_resolved=%v",
		targetPO.ID, r.PurchaseOrder.Title, r.PurchaseOrder.Status, targetPO.VendorID, targetPO.ProjectID,
		r.Vendor != nil, r.Project != nil)
}

// TestE2E_FindPurchaseOrder_ByVendorName_Strict は VendorName モード。
// vendor 0 件のためスキップ。
func TestE2E_FindPurchaseOrder_ByVendorName_Strict(t *testing.T) {
	t.Skip("vendors 0 件; pending re-verification (vendor data not yet populated in this BOARD account)")
}

// TestE2E_FindPurchaseOrder_ByProjectName_Strict は ProjectName モード。
// cache-warm が必要なためスキップ（全 project 走査 → 全件 PO 個別 fetch の連鎖を防ぐ）。
func TestE2E_FindPurchaseOrder_ByProjectName_Strict(t *testing.T) {
	t.Skip("ProjectName mode requires pre-warmed cache; skipping to avoid full-fetch timeout. " +
		"Run after cache is warm: go test -tags e2e -timeout 30m -run TestE2E_FindPurchaseOrder_ByProjectName_Strict ./internal/service/find/")
}

// TestE2E_FindPurchaseOrder_ByText_Strict は Text モードで PurchaseOrder データ待ちのため SKIP。
func TestE2E_FindPurchaseOrder_ByText_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1 件取得 → 0 件ならスキップ
	page, err := api.ListPurchaseOrdersPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListPurchaseOrdersPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no purchase_orders; pending re-verification")
	}
	targetPO := page.Items[0]
	if targetPO.Title == "" {
		t.Skip("first purchase_order has empty title; cannot form text query")
	}
	titleRunes := []rune(targetPO.Title)
	if len(titleRunes) < 2 {
		t.Skip("first purchase_order title too short for text search (need >= 2 runes)")
	}
	prefix := string(titleRunes[:2])

	results, err := svc.FindPurchaseOrder(ctx, find.FindPurchaseOrderQuery{
		Text:  prefix,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPurchaseOrder(Text=%q): %v", prefix, err)
	}
	t.Logf("FindPurchaseOrder(Text=%q) returned %d results", prefix, len(results))

	// 各 result は prefix を Title または Memo に含む
	for i, r := range results {
		po := r.PurchaseOrder
		containsInAny := strings.Contains(po.Title, prefix) || strings.Contains(po.Memo, prefix)
		if !containsInAny {
			t.Errorf("results[%d]: PurchaseOrder(id=%d, title=%q, memo=%q) does not contain prefix %q",
				i, po.ID, po.Title, po.Memo, prefix)
		}
	}
}

// TestE2E_FindPurchaseOrder_ByStatus_Strict は Status モードで PurchaseOrder データ待ちのため SKIP。
func TestE2E_FindPurchaseOrder_ByStatus_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 5 件取得して status を discovery する
	page, err := api.ListPurchaseOrdersPage(ctx, 1, 5)
	if err != nil {
		t.Fatalf("ListPurchaseOrdersPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no purchase_orders; pending re-verification")
	}

	statusCount := make(map[string]int)
	for _, po := range page.Items {
		if po.Status != "" {
			statusCount[po.Status]++
		}
	}
	if len(statusCount) == 0 {
		t.Skip("no purchase_order with non-empty status found in first 5 items")
	}
	var targetStatus string
	var maxCount int
	for s, c := range statusCount {
		if c > maxCount {
			maxCount = c
			targetStatus = s
		}
	}

	results, err := svc.FindPurchaseOrder(ctx, find.FindPurchaseOrderQuery{
		Status: targetStatus,
		Limit:  3,
		Opts:   e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPurchaseOrder(Status=%q): %v", targetStatus, err)
	}
	t.Logf("FindPurchaseOrder(Status=%q) returned %d results", targetStatus, len(results))

	for i, r := range results {
		if r.PurchaseOrder.Status != targetStatus {
			t.Errorf("results[%d]: PurchaseOrder.Status=%q want=%q", i, r.PurchaseOrder.Status, targetStatus)
		}
	}
}

// --- FindPayment (M30) ---

// TestE2E_FindPayment_ByID_Strict は ID モードで FindPayment が正しく Payment を返し、
// Vendor enrichment が整合していることを検証する。
//
// 当該アカウントは payments 0 件のため pending re-verification。
func TestE2E_FindPayment_ByID_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1 件取得 → 0 件ならスキップ
	page, err := api.ListPaymentsPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListPaymentsPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no payments; pending re-verification (payment data not yet populated in this BOARD account)")
	}
	targetPayment := page.Items[0]

	results, err := svc.FindPayment(ctx, find.FindPaymentQuery{
		ID:   targetPayment.ID,
		Opts: e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPayment(ID=%d): %v", targetPayment.ID, err)
	}
	if len(results) != 1 {
		t.Fatalf("FindPayment(ID=%d): expected 1 result, got %d", targetPayment.ID, len(results))
	}
	r := results[0]

	// Payment ID 一致確認
	if r.Payment.ID != targetPayment.ID {
		t.Errorf("r.Payment.ID: got=%d want=%d", r.Payment.ID, targetPayment.ID)
	}

	// Vendor enrichment: VendorID 非ゼロなら nil 不可
	if targetPayment.VendorID != 0 {
		if r.Vendor == nil {
			t.Errorf("r.Vendor is nil but Payment.VendorID=%d (enrichment missing)", targetPayment.VendorID)
		} else if r.Vendor.ID != targetPayment.VendorID {
			t.Errorf("r.Vendor.ID: got=%d want=%d", r.Vendor.ID, targetPayment.VendorID)
		} else {
			t.Logf("Vendor enrichment OK: id=%d", r.Vendor.ID)
		}
	}

	t.Logf("FindPayment(ID=%d): status=%q vendor_id=%d purchase_order_id=%d vendor_resolved=%v",
		targetPayment.ID, r.Payment.Status, targetPayment.VendorID, targetPayment.PurchaseOrderID,
		r.Vendor != nil)
}

// TestE2E_FindPayment_ByVendorName_Strict は VendorName モード。
// vendor 0 件のためスキップ。
func TestE2E_FindPayment_ByVendorName_Strict(t *testing.T) {
	t.Skip("vendors 0 件; pending re-verification (vendor data not yet populated in this BOARD account)")
}

// TestE2E_FindPayment_ByPurchaseOrderID_Strict は PurchaseOrderID モード。
// purchase_orders 0 件のためスキップ。
func TestE2E_FindPayment_ByPurchaseOrderID_Strict(t *testing.T) {
	t.Skip("purchase_orders 0 件; pending re-verification (purchase_order data not yet populated in this BOARD account)")
}

// TestE2E_FindPayment_ByText_Strict は Text モードで Payment データ待ちのため SKIP。
func TestE2E_FindPayment_ByText_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 1 件取得 → 0 件ならスキップ
	page, err := api.ListPaymentsPage(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListPaymentsPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no payments; pending re-verification")
	}
	targetPayment := page.Items[0]
	if targetPayment.Memo == "" {
		t.Skip("first payment has empty memo; cannot form text query")
	}
	memoRunes := []rune(targetPayment.Memo)
	if len(memoRunes) < 2 {
		t.Skip("first payment memo too short for text search (need >= 2 runes)")
	}
	prefix := string(memoRunes[:2])

	results, err := svc.FindPayment(ctx, find.FindPaymentQuery{
		Text:  prefix,
		Limit: 5,
		Opts:  e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPayment(Text=%q): %v", prefix, err)
	}
	t.Logf("FindPayment(Text=%q) returned %d results", prefix, len(results))

	// 各 result は prefix を Memo に含む
	for i, r := range results {
		p := r.Payment
		if !strings.Contains(p.Memo, prefix) {
			t.Errorf("results[%d]: Payment(id=%d, memo=%q) does not contain prefix %q",
				i, p.ID, p.Memo, prefix)
		}
	}
}

// TestE2E_FindPayment_ByStatus_Strict は Status モードで Payment データ待ちのため SKIP。
func TestE2E_FindPayment_ByStatus_Strict(t *testing.T) {
	svc, api := newE2EFindService(t)
	ctx := context.Background()

	// 5 件取得して status を discovery する
	page, err := api.ListPaymentsPage(ctx, 1, 5)
	if err != nil {
		t.Fatalf("ListPaymentsPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Skip("no payments; pending re-verification")
	}

	statusCount := make(map[string]int)
	for _, p := range page.Items {
		if p.Status != "" {
			statusCount[p.Status]++
		}
	}
	if len(statusCount) == 0 {
		t.Skip("no payment with non-empty status found in first 5 items")
	}
	var targetStatus string
	var maxCount int
	for s, c := range statusCount {
		if c > maxCount {
			maxCount = c
			targetStatus = s
		}
	}

	results, err := svc.FindPayment(ctx, find.FindPaymentQuery{
		Status: targetStatus,
		Limit:  3,
		Opts:   e2eOpts(),
	})
	if err != nil {
		t.Fatalf("FindPayment(Status=%q): %v", targetStatus, err)
	}
	t.Logf("FindPayment(Status=%q) returned %d results", targetStatus, len(results))

	for i, r := range results {
		if r.Payment.Status != targetStatus {
			t.Errorf("results[%d]: Payment.Status=%q want=%q", i, r.Payment.Status, targetStatus)
		}
	}
}
