//go:build e2e

package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// T05: ClientID lookup 正常系
func TestE2E_FindProject_ByClientID_Returns_NonEmpty(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1 件 client を取得
	cs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(cs) == 0 {
		t.Skipf("[SKIP:no-data] clients seed: err=%v rs=%d", err, len(cs))
	}
	cid := cs[0].Client.ID

	ps, err := svc.FindProject(ctx, find.FindProjectQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		ClientID:       cid,
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindProject(ClientID=%d): %v", cid, err)
	}
	skipIfNoData(t, "projects for client", len(ps), 1)
	for _, p := range ps {
		if p.Project.Client == nil || p.Project.Client.ID != cid {
			t.Errorf("expected client.id=%d, got=%v", cid, p.Project.Client)
		}
	}
}

// T06: Project Search via ClientID seed
// project name 自身に "株" が含まれる前提を避けるため、client → ClientID 経由で取得する。
func TestE2E_FindProject_ByClientSeeded_Returns_NonEmpty(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(cs) == 0 {
		t.Skipf("[SKIP:no-data] client seed: err=%v rs=%d", err, len(cs))
	}

	ps, err := svc.FindProject(ctx, find.FindProjectQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		ClientID:       cs[0].Client.ID,
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindProject(ClientID): %v", err)
	}
	skipIfNoData(t, "projects for client", len(ps), 1)
}

// T07: Statuses post-filter (OR 評価) — smoke test
// 本ケースはエラーにならず統計が出ることを確認する smoke で、
// post-filter のフィルタ精度（OR 評価）は unit test (filter_test.go) で網羅済み。
// FindProject の Statuses は post-filter（OrderStatusName / DeliveryStatusName を OR）
func TestE2E_FindProject_StatusesPostFilter(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil || len(cs) == 0 {
		t.Skipf("[SKIP:no-data] client seed: err=%v rs=%d", err, len(cs))
	}

	ps, err := svc.FindProject(ctx, find.FindProjectQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		ClientID:       cs[0].Client.ID,
		Statuses:       []string{"受注", "完了"},
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindProject(Statuses): %v", err)
	}
	t.Logf("projects with status filter: count=%d", len(ps))
}

// T08: ClientName 重複ヒット時の disambiguate error は MCP handler 経由で検証 (T45)。
// Service 層には ClientName フィールドが存在しないため、本ケースは ResolveClientByName を直接叩いて検証する。
func TestE2E_FindProject_ResolveClientByName_AmbiguityError(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// "株" は通常複数 client 名にマッチする（重複前提）
	id, err := svc.ResolveClientByName(ctx, "株", repository.ReadOptions{})
	if skipIfRateLimit(t, err) {
		return
	}
	if err == nil {
		// 単一一致した場合は ambiguity でないので SKIP
		t.Skipf("[SKIP:no-data] '株' resolved uniquely to id=%d (need duplicate matches)", id)
	}
	if !containsErrSubstr(err, "ambiguous") && !containsErrSubstr(err, "multiple") {
		t.Fatalf("expected ambiguity error, got: %v", err)
	}
	t.Logf("disambiguate error confirmed: %v", err)
}

// T09: Status-only クエリ reject (narrowing 必須)
func TestE2E_FindProject_StatusOnly_Rejects(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := svc.FindProject(ctx, find.FindProjectQuery{
		Status: "受注",
	})
	if err == nil {
		t.Fatalf("expected reject for status-only query")
	}
	if !containsErrSubstr(err, "narrow") {
		t.Fatalf("expected narrowing error, got: %v", err)
	}
}
