//go:build e2e

package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T01: ID lookup 正常系
func TestE2E_FindClient_ByID_Returns_Single(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// シード: Name="株" で 1 件取得して ID を得る
	rs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Skipf("[SKIP:no-data] FindClient seed: %v", err)
	}
	skipIfNoData(t, "clients", len(rs), 1)

	id := rs[0].Client.ID
	if id <= 0 {
		t.Fatalf("expected positive client ID, got %d", id)
	}

	got, err := svc.FindClient(ctx, find.FindClientQuery{ID: id})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindClient(ID=%d) failed: %v", id, err)
	}
	if len(got) != 1 || got[0].Client.ID != id {
		t.Fatalf("FindClient(ID=%d): unexpected result %+v", id, got)
	}
}

// T02: NameCont enrichment 正常系
func TestE2E_FindClient_ByName_Enriches_BranchesContacts(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 適当な 1 文字を Name に与え substring 検索
	rs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindClient by Name: %v", err)
	}
	skipIfNoData(t, `clients matching Name="株"`, len(rs), 1)

	// enrichment は non-fatal なので branches/contacts は 0 件もあり得る
	t.Logf("FindClient enriched: clients=%d branches[0]=%d contacts[0]=%d",
		len(rs), len(rs[0].Branches), len(rs[0].Contacts))
}

// T03: 重複候補 (NameCont で複数件返る境界)
func TestE2E_FindClient_NameCont_MultipleMatches(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 10},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindClient: %v", err)
	}
	if len(rs) < 2 {
		t.Skipf("[SKIP:no-data] need >=2 clients matching '株', got=%d", len(rs))
	}
	t.Logf("multiple matches: count=%d (FindClient does not disambiguate; resolver does)", len(rs))
}

// T04: ID + Name 両方指定は Name が無視される（priority: ID > Name > Text）
// 本ケースは reject ではなく "ID 優先で 1 件返る" 仕様の確認。
func TestE2E_FindClient_IDDominatesName(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	skipIfNoData(t, "clients", len(rs), 1)
	id := rs[0].Client.ID

	// 存在しない名前を Name に指定しても ID が優先されること
	got, err := svc.FindClient(ctx, find.FindClientQuery{
		ID:   id,
		Name: "ZZZ_NEVER_MATCH_ZZZ",
	})
	if err != nil {
		t.Fatalf("FindClient(ID + bogus Name): %v", err)
	}
	if len(got) != 1 || got[0].Client.ID != id {
		t.Fatalf("expected ID precedence: got=%+v", got)
	}
}
