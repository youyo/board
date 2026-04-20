//go:build e2e

package boardapi_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

const (
	e2eBaseURL = "https://api.the-board.jp"
	e2eTimeout = 30 * time.Second
)

// skipIfNoCredentials skips the test if BOARD_API_KEY or BOARD_API_TOKEN is not set.
func skipIfNoCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
		t.Skip("E2E: BOARD_API_KEY and BOARD_API_TOKEN are required")
	}
}

// newE2EClient returns a boardapi.Client configured with real credentials from environment variables.
func newE2EClient(t *testing.T) *boardapi.Client {
	t.Helper()
	skipIfNoCredentials(t)
	return boardapi.New(
		e2eBaseURL,
		os.Getenv("BOARD_API_KEY"),
		os.Getenv("BOARD_API_TOKEN"),
		e2eTimeout,
	)
}

// skipIfNotFound skips the test when err is a boardapi 404 Not Found error.
// Use this for resources that may not be enabled for all BOARD accounts.
func skipIfNotFound(t *testing.T, err error, context string) {
	t.Helper()
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorNotFound {
		t.Skipf("E2E: %s returned 404 (resource not available for this account)", context)
	}
}

// requirePositiveID asserts that id > 0.
func requirePositiveID(t *testing.T, id int, label string) {
	t.Helper()
	if id <= 0 {
		t.Fatalf("%s: expected positive ID, got %d", label, id)
	}
}

// requireNonEmpty asserts that s is not empty.
func requireNonEmpty(t *testing.T, s string, label string) {
	t.Helper()
	if s == "" {
		t.Fatalf("%s: expected non-empty string", label)
	}
}

// skipIfRateLimit skips the test when err is a boardapi 429 Rate Limit error.
// E2E tests that issue many paginated API calls may hit the 3/sec rate limit.
func skipIfRateLimit(t *testing.T, err error, context string) {
	t.Helper()
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorRateLimit {
		t.Skipf("E2E: %s hit rate limit (429); skipping", context)
	}
}

// dumpJSON は BOARD API の生レスポンスを tmp/e2e-artifacts/{resource}_{id}.json に書き出す。
// 配置先は repo root 直下の tmp/（.gitignore 済み）。M01 の 厳格フィールド突合 と
// 組み合わせて、未マップフィールドが出た際に「実 API のペイロードが何だったか」を
// レビュー可能にするための副産物。書き込み失敗はテストを fail させない（best-effort）。
//
// 使用例:
//
//	raw, _ := client.ListClientsRaw(ctx)
//	dumpJSON(t, "clients", 0, raw) // list なら id=0
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

// findRepoRoot は CWD から go.mod を find-up して repo root を返す。
// `go test ./internal/boardapi/...` で CWD が package ディレクトリになるため、
// repo root を明示的に発見する必要がある。
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
