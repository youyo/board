package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
)

func TestConfigurePathCmd(t *testing.T) {
	t.Run("BOARD_CONFIG_PATH が設定されている場合その値が返る", func(t *testing.T) {
		dir := t.TempDir()
		expected := filepath.Join(dir, "config.toml")
		t.Setenv("BOARD_CONFIG_PATH", expected)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}
