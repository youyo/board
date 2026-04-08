package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureCurrentProfileCmd(t *testing.T) {
	t.Run("default config returns default profile", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "current-profile")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != "default" {
			t.Errorf("expected %q, got %q", "default", got)
		}
	})

	t.Run("after SetCurrentProfile, the new name is returned", func(t *testing.T) {
		path := newTempConfig(t)

		cfg := config.DefaultConfig()
		config.AddOrUpdateProfile(&cfg, "production", config.DefaultProfileConfig())
		config.SetCurrentProfile(&cfg, "production")
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "current-profile")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != "production" {
			t.Errorf("expected %q, got %q", "production", got)
		}
	})
}
