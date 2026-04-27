//go:build e2e

// Package find E2E test helpers (build tag: e2e).
//
// 設計方針:
//   - Service 層を直接呼び出す per-batch 実行型 E2E。HTTP MCP server 起動は伴わない。
//   - SKIP テンプレート 4 種を統一フォーマット `[SKIP:cat] message` で提供（CI ログ grep 用）。
//   - newE2EService は app.New("") で全 22 リソースの repository を組み立てる。
//   - 並列実行は禁止（BOARD API 3 req/sec rate limit、t.Parallel() を呼ばない）。
//
// 実行例 (per-batch):
//
//	go test -tags e2e -v -count=1 -run TestE2E_FindClient   ./internal/service/find/
//	go test -tags e2e -v -count=1 -run TestE2E_FindProject  ./internal/service/find/
//
// CI では実行しない（README / docs/specs §39 参照）。
package find_test

import (
	"errors"
	"os"
	"testing"

	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
)

// skipIfNoCreds skips the test if BOARD credentials are not set.
// SKIP ログ: `[SKIP:no-creds] ...`
func skipIfNoCreds(t *testing.T) {
	t.Helper()
	if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
		t.Skipf("[SKIP:no-creds] BOARD_API_KEY and BOARD_API_TOKEN required")
	}
}

// skipIfNoData skips when actual count is below required threshold.
// SKIP ログ: `[SKIP:no-data] {label} got=N want>=M`
func skipIfNoData(t *testing.T, label string, got, want int) {
	t.Helper()
	if got < want {
		t.Skipf("[SKIP:no-data] %s got=%d want>=%d", label, got, want)
	}
}

// skipIfCacheWarmNeeded skips when cache must be pre-populated.
// SKIP ログ: `[SKIP:cache-warm] ...`
func skipIfCacheWarmNeeded(t *testing.T, reason string) {
	t.Helper()
	t.Skipf("[SKIP:cache-warm] %s", reason)
}

// skipIfRateLimit skips if err is a 429 rate-limit error and returns true.
// SKIP ログ: `[SKIP:rate-limit] ...`
func skipIfRateLimit(t *testing.T, err error) bool {
	t.Helper()
	if err == nil {
		return false
	}
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorRateLimit {
		t.Skipf("[SKIP:rate-limit] %v", err)
		return true
	}
	return false
}

// newE2EService initializes a real find.Service backed by the BOARD API
// (via app.New("")). It uses the user's normal cache DB; tests should not
// mutate state. Caller is responsible for app.Close() via t.Cleanup.
func newE2EService(t *testing.T) *find.Service {
	t.Helper()
	skipIfNoCreds(t)
	a, err := app.New("")
	if err != nil {
		t.Fatalf("newE2EService: app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a.FindService()
}

// containsErrSubstr returns true if err contains substring s (case sensitive).
// Used for reject case verification with relaxed matching (error wording may shift).
func containsErrSubstr(err error, s string) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), s)
}

// contains reports whether substr is within s.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
