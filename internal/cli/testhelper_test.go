package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// newTempConfig sets up a temporary config path for testing and
// configures the BOARD_CONFIG_PATH environment variable to use it.
func newTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("BOARD_CONFIG_PATH", path)
	return path
}

// executeCmd executes a command and returns its stdout content.
func executeCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	_, err := cmd.ExecuteC()
	return buf.String(), err
}
