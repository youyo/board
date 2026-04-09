//go:build e2e

package boardapi_test

import (
	"errors"
	"os"
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
