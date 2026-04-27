package cli_test

import (
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureSetCmd(t *testing.T) {
	t.Run("setting timezone is reflected in config.toml", func(t *testing.T) {
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

	t.Run("setting profiles.default.api_key saves in plain text", func(t *testing.T) {
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

	t.Run("an invalid key returns an error", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "unknown_key", "value")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("a single argument returns a cobra args error", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "set", "timezone")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
