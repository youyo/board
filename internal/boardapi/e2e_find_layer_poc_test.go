//go:build e2e

// E2E PoC tests for find2 layer: projects ListProjects with response_group scale measurement.
//
// 目的: Document 4 種（estimate/order/delivery/receipt）の逆マッピング戦略で使用する
// projects.ListProjects(ResponseGroup=xxx) の cold start スケール実測。
//
// 各テストは独立実行（rate limit 配慮）。テスト間は手動 sleep 10s を推奨。
//
// Usage (per test, rate-limit safe):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... \
//	  go test -tags e2e -v -count=1 \
//	    -run TestE2E_FindLayerPoC_Projects_RG_Estimate_ScaleCold \
//	    ./internal/boardapi/
package boardapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// preflightBudget は実測前に rate limit 残量を確認する。
// X-Ratelimit-Remaining が 2500 未満の場合はテストを SKIP する。
// ヘッダーが取得できない場合は warn ログのみ出して継続。
func preflightBudget(t *testing.T, client *boardapi.Client) {
	t.Helper()
	ctx := context.Background()
	result, err := client.ListClients(ctx, boardapi.ClientListOptions{PerPage: 1})
	if err != nil {
		skipIfRateLimit(t, err, "preflightBudget")
		t.Logf("[WARN] preflightBudget: ListClients(per_page=1) error: %v — continuing anyway", err)
		return
	}
	remaining := result.Meta.RateLimitRemaining
	if remaining == 0 {
		// ヘッダーが取得できなかった場合（remaining=0 はゼロ値 → 判定不能）
		t.Logf("[WARN] preflightBudget: X-RateLimit-Remaining not found in headers — continuing without budget check")
		return
	}
	t.Logf("[preflight] X-RateLimit-Remaining=%d", remaining)
	if remaining < 2500 {
		t.Skipf("E2E: rate limit budget insufficient (remaining=%d < 2500); defer 1 day", remaining)
	}
}

// TestE2E_FindLayerPoC_Projects_RG_Estimate_ScaleCold は
// projects.ListProjects(ResponseGroup="estimate") の cold start スケールを実測する。
// retry 発動が 1 回以上あった場合は計測値が無効のため fail する。
func TestE2E_FindLayerPoC_Projects_RG_Estimate_ScaleCold(t *testing.T) {
	client := newE2EClient(t)
	preflightBudget(t, client)

	ctx, retryCounter := boardapi.WithRetryCounter(context.Background())

	start := time.Now()
	result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{
		ResponseGroup: "estimate",
		PerPage:       100,
	})
	elapsed := time.Since(start)

	skipIfRateLimit(t, err, "ListProjects RG=estimate")
	if err != nil {
		t.Fatalf("ListProjects RG=estimate: %v", err)
	}

	itemCount := len(result.Items)
	// pagination 回数は ceil(itemCount / 100) で推定（実際のページ数は ListAllWithResult が内部処理）
	pages := (itemCount + 99) / 100
	if pages == 0 {
		pages = 1
	}
	retryCount := retryCounter.Load()

	t.Logf("[PoC] cold_latency=%dms pages=%d items=%d retry_count=%d response_group=estimate",
		elapsed.Milliseconds(), pages, itemCount, retryCount)

	if retryCount > 0 {
		t.Errorf("retry fired %d times — scale measurement invalid; rerun after 30min", retryCount)
	}
}

// TestE2E_FindLayerPoC_Projects_RG_Order_ScaleCold は
// projects.ListProjects(ResponseGroup="order") の cold start スケールを実測する。
func TestE2E_FindLayerPoC_Projects_RG_Order_ScaleCold(t *testing.T) {
	client := newE2EClient(t)
	preflightBudget(t, client)

	ctx, retryCounter := boardapi.WithRetryCounter(context.Background())

	start := time.Now()
	result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{
		ResponseGroup: "order",
		PerPage:       100,
	})
	elapsed := time.Since(start)

	skipIfRateLimit(t, err, "ListProjects RG=order")
	if err != nil {
		t.Fatalf("ListProjects RG=order: %v", err)
	}

	itemCount := len(result.Items)
	pages := (itemCount + 99) / 100
	if pages == 0 {
		pages = 1
	}
	retryCount := retryCounter.Load()

	t.Logf("[PoC] cold_latency=%dms pages=%d items=%d retry_count=%d response_group=order",
		elapsed.Milliseconds(), pages, itemCount, retryCount)

	if retryCount > 0 {
		t.Errorf("retry fired %d times — scale measurement invalid; rerun after 30min", retryCount)
	}
}

// TestE2E_FindLayerPoC_Projects_RG_Delivery_ScaleCold は
// projects.ListProjects(ResponseGroup="delivery") の cold start スケールを実測する。
func TestE2E_FindLayerPoC_Projects_RG_Delivery_ScaleCold(t *testing.T) {
	client := newE2EClient(t)
	preflightBudget(t, client)

	ctx, retryCounter := boardapi.WithRetryCounter(context.Background())

	start := time.Now()
	result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{
		ResponseGroup: "delivery",
		PerPage:       100,
	})
	elapsed := time.Since(start)

	skipIfRateLimit(t, err, "ListProjects RG=delivery")
	if err != nil {
		t.Fatalf("ListProjects RG=delivery: %v", err)
	}

	itemCount := len(result.Items)
	pages := (itemCount + 99) / 100
	if pages == 0 {
		pages = 1
	}
	retryCount := retryCounter.Load()

	t.Logf("[PoC] cold_latency=%dms pages=%d items=%d retry_count=%d response_group=delivery",
		elapsed.Milliseconds(), pages, itemCount, retryCount)

	if retryCount > 0 {
		t.Errorf("retry fired %d times — scale measurement invalid; rerun after 30min", retryCount)
	}
}

// TestE2E_FindLayerPoC_Projects_RG_Receipt_ScaleCold は
// projects.ListProjects(ResponseGroup="receipt") の cold start スケールを実測する。
func TestE2E_FindLayerPoC_Projects_RG_Receipt_ScaleCold(t *testing.T) {
	client := newE2EClient(t)
	preflightBudget(t, client)

	ctx, retryCounter := boardapi.WithRetryCounter(context.Background())

	start := time.Now()
	result, err := client.ListProjects(ctx, boardapi.ProjectListOptions{
		ResponseGroup: "receipt",
		PerPage:       100,
	})
	elapsed := time.Since(start)

	skipIfRateLimit(t, err, "ListProjects RG=receipt")
	if err != nil {
		t.Fatalf("ListProjects RG=receipt: %v", err)
	}

	itemCount := len(result.Items)
	pages := (itemCount + 99) / 100
	if pages == 0 {
		pages = 1
	}
	retryCount := retryCounter.Load()

	t.Logf("[PoC] cold_latency=%dms pages=%d items=%d retry_count=%d response_group=receipt",
		elapsed.Milliseconds(), pages, itemCount, retryCount)

	if retryCount > 0 {
		t.Errorf("retry fired %d times — scale measurement invalid; rerun after 30min", retryCount)
	}
}
