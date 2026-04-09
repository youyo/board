package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/board/internal/app"
)

// newTestApp is a helper that creates a test App.
// It writes a minimal config.toml to a temporary directory and uses a temporary file for the DB.
func newTestApp(t *testing.T) *app.App {
	t.Helper()

	tmpDir := t.TempDir()

	// Write a minimal config.toml
	cfgContent := `
current_profile = "default"
timezone = "Asia/Tokyo"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "test-api-key"
api_token = "test-api-token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)

	// Set DB to a temporary file
	dbPath := filepath.Join(tmpDir, "cache.db")
	t.Setenv("BOARD_CACHE_PATH", dbPath)

	a, err := app.New("")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestNew_DefaultProfile(t *testing.T) {
	a := newTestApp(t)
	if a == nil {
		t.Fatal("expected non-nil App")
	}
	if a.Repos == nil {
		t.Fatal("expected non-nil Repos")
	}
}

func TestNew_ExplicitProfile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "key1"
api_token = "token1"
daily_auto_refresh = false

[profiles.prod]
base_url = "https://api.the-board.jp/v1/"
api_key = "prod-key"
api_token = "prod-token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(tmpDir, "cache.db"))
	t.Setenv("BOARD_API_KEY", "")
	t.Setenv("BOARD_API_TOKEN", "")

	a, err := app.New("prod")
	if err != nil {
		t.Fatalf("app.New(prod): %v", err)
	}
	defer func() { _ = a.Close() }()

	if a.Profile.APIKey != "prod-key" {
		t.Errorf("Profile.APIKey = %q, want %q", a.Profile.APIKey, "prod-key")
	}
}

func TestClose_CloseDB(t *testing.T) {
	a := newTestApp(t)
	// Close is also called in Cleanup, but calling it explicitly should not error.
	// (Note: double Close occurs since Cleanup also calls it, but the error is ignored.)
	if err := a.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestNew_DBOpenFail(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "key"
api_token = "token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	// Set DB path to a read-only directory to trigger open failure
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(roDir, "subdir", "cache.db"))

	_, err := app.New("")
	if err == nil {
		t.Fatal("expected error for unwritable db path, got nil")
	}
}

func TestNew_ProfileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "key"
api_token = "token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(tmpDir, "cache.db"))

	_, err := app.New("unknown")
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
}

func TestNew_InvalidTimezone(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
current_profile = "default"
timezone = "Invalid/Zone"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "key"
api_token = "token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(tmpDir, "cache.db"))

	// Invalid timezone falls back to UTC without error
	a, err := app.New("")
	if err != nil {
		t.Fatalf("expected no error with invalid timezone (fallback to UTC), got: %v", err)
	}
	defer func() { _ = a.Close() }()
}

// TestNew_EnvOverridesCredentials verifies that BOARD_API_KEY and BOARD_API_TOKEN
// environment variables override config.toml credentials.
func TestNew_EnvOverridesCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	cfgContent := `
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp/v1/"
api_key = "file-key"
api_token = "file-token"
daily_auto_refresh = false
`
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("BOARD_CONFIG_PATH", cfgPath)
	t.Setenv("BOARD_CACHE_PATH", filepath.Join(tmpDir, "cache.db"))
	t.Setenv("BOARD_API_KEY", "env-key")
	t.Setenv("BOARD_API_TOKEN", "env-token")

	a, err := app.New("")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	defer func() { _ = a.Close() }()

	if a.Profile.APIKey != "env-key" {
		t.Errorf("Profile.APIKey = %q, want %q", a.Profile.APIKey, "env-key")
	}
	if a.Profile.APIToken != "env-token" {
		t.Errorf("Profile.APIToken = %q, want %q", a.Profile.APIToken, "env-token")
	}
}
