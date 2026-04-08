package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureUseCmd(t *testing.T) {
	t.Run("specifying an existing profile updates current_profile", func(t *testing.T) {
		path := newTempConfig(t)

		cfg := config.DefaultConfig()
		config.AddOrUpdateProfile(&cfg, "staging", config.DefaultProfileConfig())
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "use", "staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the saved config.
		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if saved.CurrentProfile != "staging" {
			t.Errorf("expected current_profile=staging, got %q", saved.CurrentProfile)
		}
	})

	t.Run("specifying a non-existent profile returns an error", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "use", "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "profile not found") {
			t.Errorf("expected profile not found error, got: %v", err)
		}
	})

	t.Run("no arguments returns a cobra args error", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "use")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
