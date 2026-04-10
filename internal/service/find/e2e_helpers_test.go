//go:build e2e

package find_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/service/find"
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
