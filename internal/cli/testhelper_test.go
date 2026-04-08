package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// newTempConfig はテスト用の一時 config パスをセットアップし、
// BOARD_CONFIG_PATH 環境変数でその値を使うよう設定する。
func newTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("BOARD_CONFIG_PATH", path)
	return path
}

// executeCmd はコマンドを実行し、stdout の内容を返す。
func executeCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	_, err := cmd.ExecuteC()
	return buf.String(), err
}
