package cli_test

import (
	"testing"

	"github.com/youyo/board/internal/cli"
)

func TestNewRootCmd_HasVersionFlag(t *testing.T) {
	cmd := cli.NewRootCmd("1.2.3")
	if cmd.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", cmd.Version, "1.2.3")
	}
}

func TestNewRootCmd_GlobalFlags(t *testing.T) {
	cmd := cli.NewRootCmd("dev")
	flags := []string{"profile", "refresh", "force-refresh", "pretty", "limit"}
	for _, name := range flags {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestNewRootCmd_LimitDefault(t *testing.T) {
	cmd := cli.NewRootCmd("dev")
	f := cmd.PersistentFlags().Lookup("limit")
	if f == nil {
		t.Fatal("--limit flag not found")
	}
	if f.DefValue != "50" {
		t.Errorf("--limit default = %q, want %q", f.DefValue, "50")
	}
}

func TestNewRootCmd_Configure_SkipAppInit(t *testing.T) {
	// configure サブコマンドを実行しても PersistentPreRunE が App 初期化しないことを確認。
	// configure は設定ファイルがなくても動作する必要がある。
	cmd := cli.NewRootCmd("dev")
	// --help は PersistentPreRunE を呼ばないため configure path を使う
	cmd.SetArgs([]string{"configure", "path"})
	// configure path は設定ファイルを読まずにパスを表示するだけ
	if err := cmd.Execute(); err != nil {
		t.Errorf("configure path should not fail: %v", err)
	}
}

func TestNewRootCmd_HasConfigureSubCmd(t *testing.T) {
	cmd := cli.NewRootCmd("dev")
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "configure" {
			found = true
			break
		}
	}
	if !found {
		t.Error("configure subcommand not found in root")
	}
}
