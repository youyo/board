package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

// TestNewCacheCmd verifies the command structure (subcommands).
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
			t.Errorf("subcommand %q is not registered", want)
		}
	}
}

// TestCacheExpireCmdFlags verifies the --resource flag of cache expire.
func TestCacheExpireCmdFlags(t *testing.T) {
	var expireCmd *cobra.Command
	for _, sub := range cli.NewCacheCmd().Commands() {
		if sub.Use == "expire" {
			expireCmd = sub
		}
	}
	if expireCmd == nil {
		t.Fatal("expire command not found")
	}
	if f := expireCmd.Flags().Lookup("resource"); f == nil {
		t.Error("expire: --resource flag is not defined")
	}
}

// TestCacheClearCmdFlags verifies the --resource flag of cache clear.
func TestCacheClearCmdFlags(t *testing.T) {
	var clearCmd *cobra.Command
	for _, sub := range cli.NewCacheCmd().Commands() {
		if sub.Use == "clear" {
			clearCmd = sub
		}
	}
	if clearCmd == nil {
		t.Fatal("clear command not found")
	}
	if f := clearCmd.Flags().Lookup("resource"); f == nil {
		t.Error("clear: --resource flag is not defined")
	}
}

// TestRootCmdHasCacheSubcommand verifies that cache is registered on the root command.
func TestRootCmdHasCacheSubcommand(t *testing.T) {
	root := cli.NewRootCmd("test")
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "cache" {
			found = true
		}
	}
	if !found {
		t.Error("cache subcommand is not registered on root command")
	}
}
