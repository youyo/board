package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/board/internal/config"
)

// T01: Default Config generation
func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.CurrentProfile != "default" {
		t.Errorf("expected CurrentProfile=default, got %q", cfg.CurrentProfile)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected Timezone=UTC, got %q", cfg.Timezone)
	}
	if cfg.Profiles == nil {
		t.Error("expected Profiles to be non-nil")
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Error("expected default profile to exist")
	}
}

// T02: Default ProfileConfig generation
func TestDefaultProfileConfig(t *testing.T) {
	p := config.DefaultProfileConfig()
	if p.BaseURL != "https://api.the-board.jp" {
		t.Errorf("expected BaseURL=https://api.the-board.jp, got %q", p.BaseURL)
	}
	if !p.DailyAutoRefresh {
		t.Error("expected DailyAutoRefresh=true")
	}
	if p.RequestTimeoutSeconds != 30 {
		t.Errorf("expected RequestTimeoutSeconds=30, got %d", p.RequestTimeoutSeconds)
	}
	if p.RetryMax != 5 {
		t.Errorf("expected RetryMax=5, got %d", p.RetryMax)
	}
	if p.PrettyDefault {
		t.Error("expected PrettyDefault=false")
	}
}

// T03: Save to TOML file (verify 0600 permissions)
func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.DefaultConfig()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permission 0600, got %o", info.Mode().Perm())
	}
}

// T04: Load from TOML file
func TestLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := config.DefaultConfig()
	original.CurrentProfile = "myprofile"
	original.Timezone = "Asia/Tokyo"
	original.Profiles["myprofile"] = config.ProfileConfig{
		BaseURL:               "https://custom.api.example.com",
		APIKey:                "testkey",
		APIToken:              "testtoken",
		DailyAutoRefresh:      false,
		RequestTimeoutSeconds: 60,
		RetryMax:              3,
		PrettyDefault:         true,
	}

	if err := config.Save(original, path); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.CurrentProfile != "myprofile" {
		t.Errorf("expected CurrentProfile=myprofile, got %q", loaded.CurrentProfile)
	}
	if loaded.Timezone != "Asia/Tokyo" {
		t.Errorf("expected Timezone=Asia/Tokyo, got %q", loaded.Timezone)
	}
	p, ok := loaded.Profiles["myprofile"]
	if !ok {
		t.Fatal("expected myprofile to exist")
	}
	if p.BaseURL != "https://custom.api.example.com" {
		t.Errorf("expected BaseURL=https://custom.api.example.com, got %q", p.BaseURL)
	}
	if p.APIKey != "testkey" {
		t.Errorf("expected APIKey=testkey, got %q", p.APIKey)
	}
	if p.RequestTimeoutSeconds != 60 {
		t.Errorf("expected RequestTimeoutSeconds=60, got %d", p.RequestTimeoutSeconds)
	}
}

// T05: Save/load multiple profiles
func TestMultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.DefaultConfig()
	cfg.Profiles["readonly"] = config.ProfileConfig{
		BaseURL:               "https://api.the-board.jp",
		APIKey:                "readonly-key",
		APIToken:              "readonly-token",
		DailyAutoRefresh:      true,
		RequestTimeoutSeconds: 30,
		RetryMax:              5,
	}

	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if _, ok := loaded.Profiles["default"]; !ok {
		t.Error("expected default profile to exist")
	}
	if _, ok := loaded.Profiles["readonly"]; !ok {
		t.Error("expected readonly profile to exist")
	}
}

// T06: XDG path resolution (XDG_CONFIG_HOME set)
func TestConfigPathXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOARD_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", "/tmp/fakehome")

	got := config.ConfigPath()
	expected := filepath.Join(dir, "board", "config.toml")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// T07: XDG path resolution (XDG_CONFIG_HOME not set)
func TestConfigPathHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOARD_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	got := config.ConfigPath()
	expected := filepath.Join(dir, ".config", "board", "config.toml")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// T08: BOARD_CONFIG_PATH environment variable
func TestConfigPathEnvOverride(t *testing.T) {
	customPath := "/tmp/custom-board-config.toml"
	t.Setenv("BOARD_CONFIG_PATH", customPath)

	got := config.ConfigPath()
	if got != customPath {
		t.Errorf("expected %q, got %q", customPath, got)
	}
}

// T09: GetCurrentProfile returns the current profile config
func TestGetCurrentProfile(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.CurrentProfile = "default"
	cfg.Profiles["default"] = config.ProfileConfig{
		BaseURL:               "https://api.the-board.jp",
		RequestTimeoutSeconds: 30,
		RetryMax:              5,
	}

	p, err := config.GetCurrentProfile(cfg)
	if err != nil {
		t.Fatalf("GetCurrentProfile() failed: %v", err)
	}
	if p.BaseURL != "https://api.the-board.jp" {
		t.Errorf("expected BaseURL=https://api.the-board.jp, got %q", p.BaseURL)
	}
}

// T10: Load non-existent file returns defaults
func TestLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.toml")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() of nonexistent file should not return error, got: %v", err)
	}
	if cfg.CurrentProfile != "default" {
		t.Errorf("expected CurrentProfile=default, got %q", cfg.CurrentProfile)
	}
}

// T11: Save to non-existent directory (parent directories created automatically)
func TestSaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.toml")

	cfg := config.DefaultConfig()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save() failed to create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %q: %v", path, err)
	}
}

// E01: Invalid TOML
func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.toml")

	if err := os.WriteFile(path, []byte("this is [not valid toml ==="), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

// E03: Get non-existent profile
func TestGetCurrentProfileNotFound(t *testing.T) {
	cfg := config.Config{
		CurrentProfile: "nonexistent",
		Profiles:       map[string]config.ProfileConfig{},
	}

	_, err := config.GetCurrentProfile(cfg)
	if err == nil {
		t.Fatal("expected ErrProfileNotFound, got nil")
	}
}

// E04: CurrentProfile is empty string
func TestGetCurrentProfileEmpty(t *testing.T) {
	cfg := config.Config{
		CurrentProfile: "",
		Profiles:       map[string]config.ProfileConfig{},
	}

	_, err := config.GetCurrentProfile(cfg)
	if err == nil {
		t.Fatal("expected ErrInvalidConfig, got nil")
	}
}

// EC01: Load TOML with empty Profiles map
func TestLoadEmptyProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `current_profile = "default"
timezone = "UTC"

[profiles]
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("expected Profiles to be non-nil (empty map)")
	}
}

// ApplyDefaults: apply defaults to zero values
func TestApplyDefaults(t *testing.T) {
	p := config.ProfileConfig{} // all zero values

	result := config.ApplyDefaults(p)

	if result.BaseURL != "https://api.the-board.jp" {
		t.Errorf("expected BaseURL=https://api.the-board.jp, got %q", result.BaseURL)
	}
	if result.RequestTimeoutSeconds != 30 {
		t.Errorf("expected RequestTimeoutSeconds=30, got %d", result.RequestTimeoutSeconds)
	}
	if result.RetryMax != 5 {
		t.Errorf("expected RetryMax=5, got %d", result.RetryMax)
	}
	// DailyAutoRefresh: zero value (false) is filled in as true
	if !result.DailyAutoRefresh {
		t.Error("expected DailyAutoRefresh=true after ApplyDefaults")
	}
}

// AddOrUpdateProfile test
func TestAddOrUpdateProfile(t *testing.T) {
	cfg := config.DefaultConfig()
	newProfile := config.ProfileConfig{
		BaseURL:               "https://new.example.com",
		RequestTimeoutSeconds: 10,
	}

	config.AddOrUpdateProfile(&cfg, "new", newProfile)

	p, ok := cfg.Profiles["new"]
	if !ok {
		t.Fatal("expected new profile to exist")
	}
	if p.BaseURL != "https://new.example.com" {
		t.Errorf("expected BaseURL=https://new.example.com, got %q", p.BaseURL)
	}
}

// SetCurrentProfile test
func TestSetCurrentProfile(t *testing.T) {
	cfg := config.DefaultConfig()
	config.SetCurrentProfile(&cfg, "readonly")
	if cfg.CurrentProfile != "readonly" {
		t.Errorf("expected CurrentProfile=readonly, got %q", cfg.CurrentProfile)
	}
}


// ApplyEnvOverrides tests

func TestApplyEnvOverrides_BothSet(t *testing.T) {
	p := config.ProfileConfig{
		APIKey:   "file-key",
		APIToken: "file-token",
	}
	t.Setenv("BOARD_API_KEY", "env-key")
	t.Setenv("BOARD_API_TOKEN", "env-token")

	result := config.ApplyEnvOverrides(p)

	if result.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "env-key")
	}
	if result.APIToken != "env-token" {
		t.Errorf("APIToken = %q, want %q", result.APIToken, "env-token")
	}
}

func TestApplyEnvOverrides_OnlyKey(t *testing.T) {
	p := config.ProfileConfig{
		APIKey:   "file-key",
		APIToken: "file-token",
	}
	t.Setenv("BOARD_API_KEY", "env-key")
	t.Setenv("BOARD_API_TOKEN", "")

	result := config.ApplyEnvOverrides(p)

	if result.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "env-key")
	}
	if result.APIToken != "file-token" {
		t.Errorf("APIToken = %q, want %q (should not be overridden)", result.APIToken, "file-token")
	}
}

func TestApplyEnvOverrides_NoneSet(t *testing.T) {
	p := config.ProfileConfig{
		APIKey:   "file-key",
		APIToken: "file-token",
	}
	t.Setenv("BOARD_API_KEY", "")
	t.Setenv("BOARD_API_TOKEN", "")

	result := config.ApplyEnvOverrides(p)

	if result.APIKey != "file-key" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "file-key")
	}
	if result.APIToken != "file-token" {
		t.Errorf("APIToken = %q, want %q", result.APIToken, "file-token")
	}
}
