package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/board/internal/app"
)

// newTestApp はテスト用の App を生成するヘルパー。
// 一時ディレクトリに最小限の config.toml を書き込み、DBはテンポラリファイルを使う。
func newTestApp(t *testing.T) *app.App {
	t.Helper()

	tmpDir := t.TempDir()

	// 最小限の config.toml を書き込む
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

	// DB を一時ファイルに設定
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
	// Cleanup で Close が呼ばれるが、明示的に呼んでもエラーなし
	// (注: Cleanup でも呼ばれるため二重 Close になるが、エラーは無視)
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
	// DB パスを読み取り専用ディレクトリに設定して open 失敗を誘発
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

	// Invalid timezone は UTC フォールバックで error なし
	a, err := app.New("")
	if err != nil {
		t.Fatalf("expected no error with invalid timezone (fallback to UTC), got: %v", err)
	}
	defer func() { _ = a.Close() }()
}
