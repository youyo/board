//go:build e2e

package boardapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/config"
)

// TestE2E_Projects_M51 は projects リソースの BOARD API 準拠 E2E テスト。
// M51 で導入した ProjectListOptions / ListResult / ItemResult を実 API で検証する。
//
// 実行条件: BOARD API 認証情報が config.toml に設定済みであること。
// Rate Limit 対策: 各サブテスト間に 400ms sleep を挿入（3 req/sec 制約）。
func TestE2E_Projects_M51(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Skipf("config not available: %v", err)
	}
	profile, err := config.GetCurrentProfile(cfg)
	if err != nil {
		t.Skipf("no current profile configured: %v", err)
	}

	client := boardapi.New(
		"https://api.the-board.jp",
		profile.APIKey,
		profile.APIToken,
		30*time.Second,
	)

	ctx := context.Background()
	sleep := func() { time.Sleep(400 * time.Millisecond) }

	// E1: ListProjects() デフォルト → items 非空、Meta に情報あり
	t.Run("E1_ListDefault", func(t *testing.T) {
		result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{})
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if len(result.Items) == 0 {
			t.Error("expected non-empty items")
		}
		t.Logf("E1: total=%d items, meta.total_count=%d", len(result.Items), result.Meta.TotalCount)
	})
	sleep()

	// E2: ListProjects(NameCont="xxx") → フィルタが届く（API エラーにならない）
	t.Run("E2_NameContFilter", func(t *testing.T) {
		_, err := client.ListProjects(ctx, boardapi.ProjectListOptions{NameCont: "test"})
		if err != nil {
			t.Fatalf("ListProjects with NameCont: %v", err)
		}
		t.Log("E2: NameCont filter sent successfully")
	})
	sleep()

	// E3: ListProjects(ResponseGroup="large") → API エラーにならない
	t.Run("E3_ResponseGroupLarge", func(t *testing.T) {
		result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{ResponseGroup: "large", PerPage: 1})
		if err != nil {
			t.Fatalf("ListProjects with ResponseGroup=large: %v", err)
		}
		t.Logf("E3: ResponseGroup=large returned %d items", len(result.Items))
	})
	sleep()

	// E4: ListProjects(IncludeArchiveFlg=true) → アーカイブ件数を記録
	t.Run("E4_IncludeArchive", func(t *testing.T) {
		v := true
		result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{IncludeArchiveFlg: &v, PerPage: 1})
		if err != nil {
			t.Fatalf("ListProjects with IncludeArchiveFlg=true: %v", err)
		}
		t.Logf("E4: include_archive=true, total_count=%d", result.Meta.TotalCount)
	})
	sleep()

	// E5: UpdatedAtGteq=<未来> → 0 件返る
	t.Run("E5_UpdatedAtGteq_Future", func(t *testing.T) {
		result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{UpdatedAtGteq: "2099-01-01 00:00:00"})
		if err != nil {
			t.Fatalf("ListProjects with future UpdatedAtGteq: %v", err)
		}
		t.Logf("E5: updated_at_gteq=2099, got %d items (expect 0)", len(result.Items))
	})
	sleep()

	// E6: GetProject(id) → ItemResult で entity が返る（最初の1件で検証）
	t.Run("E6_GetProject", func(t *testing.T) {
		baseline, err := client.ListProjects(ctx, boardapi.ProjectListOptions{PerPage: 1})
		if err != nil {
			t.Skipf("ListProjects for baseline: %v", err)
		}
		if len(baseline.Items) == 0 {
			t.Skip("no projects available")
		}
		id := baseline.Items[0].ID
		sleep()

		result, err := client.GetProject(ctx, id)
		if err != nil {
			t.Fatalf("GetProject(%d): %v", id, err)
		}
		if result.Item == nil || result.Item.ID != id {
			t.Errorf("expected ID=%d, got %+v", id, result.Item)
		}
		t.Logf("E6: GetProject(%d) OK, name=%s", id, result.Item.Name)
	})
	sleep()

	// E7: GetProjectWithGroup → ItemResult で返る
	t.Run("E7_GetProjectWithGroup", func(t *testing.T) {
		baseline, err := client.ListProjects(ctx, boardapi.ProjectListOptions{PerPage: 1})
		if err != nil {
			t.Skipf("ListProjects for baseline: %v", err)
		}
		if len(baseline.Items) == 0 {
			t.Skip("no projects available")
		}
		id := baseline.Items[0].ID
		sleep()

		result, err := client.GetProjectWithGroup(ctx, id, "estimate")
		if err != nil {
			t.Fatalf("GetProjectWithGroup(%d, estimate): %v", id, err)
		}
		if result.Item == nil || result.Item.ID != id {
			t.Errorf("expected ID=%d, got %+v", id, result.Item)
		}
		t.Logf("E7: GetProjectWithGroup(%d, estimate) OK, Estimate=%v", id, result.Item.Estimate != nil)
	})
	sleep()

	// E8: ヘッダー名の dump
	t.Run("E8_HeaderNames", func(t *testing.T) {
		result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{PerPage: 1})
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		t.Logf("E8: Meta=%+v", result.Meta)
		for k, v := range result.Headers {
			if k != "" {
				t.Logf("  header %s: %v", k, v)
			}
		}
	})
}
