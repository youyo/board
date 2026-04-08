package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureListProfilesCmd(t *testing.T) {
	t.Run("a single profile returns its name", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "list-profiles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 1 || lines[0] != "default" {
			t.Errorf("expected [default], got %v", lines)
		}
	})

	t.Run("multiple profiles return all names sorted and newline-separated", func(t *testing.T) {
		path := newTempConfig(t)

		cfg := config.DefaultConfig()
		config.AddOrUpdateProfile(&cfg, "readonly", config.DefaultProfileConfig())
		config.AddOrUpdateProfile(&cfg, "production", config.DefaultProfileConfig())
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "list-profiles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		expected := []string{"default", "production", "readonly"}
		if len(lines) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, lines)
		}
		for i, l := range lines {
			if l != expected[i] {
				t.Errorf("line[%d]: expected %q, got %q", i, expected[i], l)
			}
		}
	})
}
