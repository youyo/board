package cli_test

import (
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureSetCmd(t *testing.T) {
	t.Run("timezone を set すると config.toml に反映される", func(t *testing.T) {
		path := newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "timezone", "Asia/Tokyo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if saved.Timezone != "Asia/Tokyo" {
			t.Errorf("expected Asia/Tokyo, got %q", saved.Timezone)
		}
	})

	t.Run("profiles.default.api_key を set すると平文で保存される", func(t *testing.T) {
		path := newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "profiles.default.api_key", "mysecretkey123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		prof, ok := saved.Profiles["default"]
		if !ok {
			t.Fatal("default profile not found")
		}
		if prof.APIKey != "mysecretkey123" {
			t.Errorf("expected mysecretkey123, got %q", prof.APIKey)
		}
	})

	t.Run("profiles.default.daily_auto_refresh を false に set できる", func(t *testing.T) {
		path := newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "profiles.default.daily_auto_refresh", "false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		prof, ok := saved.Profiles["default"]
		if !ok {
			t.Fatal("default profile not found")
		}
		if prof.DailyAutoRefresh != false {
			t.Errorf("expected false, got %v", prof.DailyAutoRefresh)
		}
	})

	t.Run("profiles.default.daily_auto_refresh を true に set できる", func(t *testing.T) {
		path := newTempConfig(t)

		// まず false に設定
		cfg := config.DefaultConfig()
		prof := config.DefaultProfileConfig()
		prof.DailyAutoRefresh = false
		config.AddOrUpdateProfile(&cfg, "default", prof)
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "profiles.default.daily_auto_refresh", "true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		savedProf, ok := saved.Profiles["default"]
		if !ok {
			t.Fatal("default profile not found")
		}
		if !savedProf.DailyAutoRefresh {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("不正なキーはエラー", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "unknown_key", "value")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("引数1個は cobra の args エラー", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "timezone")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
