package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

// TestNewCacheCmd はコマンド構造（サブコマンド）を検証する。
func TestNewCacheCmd(t *testing.T) {
	cmd := cli.NewCacheCmd()

	if cmd.Use != "cache" {
		t.Errorf("Use = %q, want %q", cmd.Use, "cache")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	for _, want := range []string{"status", "expire", "clear", "path"} {
		if !subNames[want] {
			t.Errorf("サブコマンド %q が登録されていない", want)
		}
	}
}

// TestCacheExpireCmdFlags は cache expire の --resource フラグを検証する。
func TestCacheExpireCmdFlags(t *testing.T) {
	var expireCmd *cobra.Command
	for _, sub := range cli.NewCacheCmd().Commands() {
		if sub.Use == "expire" {
			expireCmd = sub
		}
	}
	if expireCmd == nil {
		t.Fatal("expire コマンドが見つからない")
	}
	if f := expireCmd.Flags().Lookup("resource"); f == nil {
		t.Error("expire: --resource フラグが定義されていない")
	}
}

// TestCacheClearCmdFlags は cache clear の --resource フラグを検証する。
func TestCacheClearCmdFlags(t *testing.T) {
	var clearCmd *cobra.Command
	for _, sub := range cli.NewCacheCmd().Commands() {
		if sub.Use == "clear" {
			clearCmd = sub
		}
	}
	if clearCmd == nil {
		t.Fatal("clear コマンドが見つからない")
	}
	if f := clearCmd.Flags().Lookup("resource"); f == nil {
		t.Error("clear: --resource フラグが定義されていない")
	}
}

// TestRootCmdHasCacheSubcommand は root コマンドに cache が登録されているか検証する。
func TestRootCmdHasCacheSubcommand(t *testing.T) {
	root := cli.NewRootCmd("test")
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "cache" {
			found = true
		}
	}
	if !found {
		t.Error("root コマンドに cache サブコマンドが登録されていない")
	}
}
