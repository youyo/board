//go:build e2e

package find_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
	"github.com/youyo/board/internal/testhelper"
)

// skipIfRateLimit skips the test when err is a boardapi 429 Rate Limit error.
// E2E tests that issue many API calls may hit the 3/sec rate limit.
func skipIfRateLimit(t *testing.T, err error, context string) {
	t.Helper()
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorRateLimit {
		t.Skipf("E2E: %s hit rate limit (429); skipping", context)
	}
}

// skipIfNoCredentials skips the test if BOARD_API_KEY or BOARD_API_TOKEN is not set.
func skipIfNoCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
		t.Skip("E2E: BOARD_API_KEY and BOARD_API_TOKEN are required")
	}
}

// newE2EApp creates a fully wired App using real API credentials from environment variables.
// config.toml is written to a temp directory; credentials come from BOARD_API_KEY / BOARD_API_TOKEN.
func newE2EApp(t *testing.T) *app.App {
	t.Helper()
	skipIfNoCredentials(t)

	tmpDir := t.TempDir()

	// Minimal config: credentials will be injected via ApplyEnvOverrides from env vars.
	cfgContent := `
current_profile = "default"

[profiles.default]
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(tmpDir, "cache.db"))

	a, err := app.New("")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// newE2EFindService creates a find.Service backed by real repositories from app.New().
func newE2EFindService(t *testing.T) (*find.Service, *boardapi.Client) {
	t.Helper()
	a := newE2EApp(t)
	repos := a.Repos
	svc := find.New(find.Repos{
		Clients:        repos.Clients,
		ClientBranches: repos.ClientBranches,
		Contacts:       repos.Contacts,
		Projects:       repos.Projects,
		Estimates:      repos.Estimates,
		Invoices:       repos.Invoices,
		Orders:         repos.Orders,
		Deliveries:     repos.Deliveries,
		Receipts:       repos.Receipts,
		Vendors:        repos.Vendors,
		VendorBranches: repos.VendorBranches,
		VendorContacts: repos.VendorContacts,
		PurchaseOrders: repos.PurchaseOrders,
		Payments:       repos.Payments,
		Users:          repos.Users,
		Groups:         repos.Groups,
	})
	return svc, a.APIClient
}

// dumpJSON は BOARD API の生レスポンスを tmp/e2e-artifacts/{resource}_{id}.json に書き出す。
// boardapi 側の e2e helper と同じパッケージ跨ぎの複製（テストパッケージ境界のため DRY 不可）。
// M01 の 厳格フィールド突合 と組み合わせて使う。失敗は best-effort で t.Log のみ。
func dumpJSON(t *testing.T, resource string, id int, raw []byte) {
	t.Helper()
	if len(raw) == 0 {
		return
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Logf("dumpJSON: repo root not found: %v", err)
		return
	}
	dir := filepath.Join(root, "tmp", "e2e-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("dumpJSON: mkdir %s: %v", dir, err)
		return
	}
	name := fmt.Sprintf("%s_%d.json", resource, id)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Logf("dumpJSON: write %s: %v", path, err)
		return
	}
}

// strictFieldDiff は生 JSON と Go struct の json タグを突合し、
// 未マップフィールドを返す。boardapi 層の E2E helper と同パターン（パッケージ境界のため複製）。
func strictFieldDiff(t *testing.T, raw []byte, target any) []string {
	t.Helper()
	return testhelper.StrictFieldDiff(t, raw, target)
}

// projectIDOrZero は project が nil でなければ ID を、nil なら 0 を返す。
func projectIDOrZero(p *boardapi.ProjectEntity) int {
	if p == nil {
		return 0
	}
	return p.ID
}

// clientIDOrZero は client が nil でなければ ID を、nil なら 0 を返す。
func clientIDOrZero(c *boardapi.ClientEntity) int {
	if c == nil {
		return 0
	}
	return c.ID
}

// findRepoRoot は CWD から go.mod を find-up して repo root を返す。
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from any ancestor of cwd")
		}
		dir = parent
	}
}
