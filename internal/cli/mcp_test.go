package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

func TestNewMCPCmd_HasServeSubCmd(t *testing.T) {
	root := cli.NewRootCmd("test")

	// Find mcp command
	var mcpCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "mcp" {
			mcpCmd = c
			break
		}
	}
	if mcpCmd == nil {
		t.Fatal("mcp command not found")
	}

	// Find serve subcommand
	var found bool
	for _, c := range mcpCmd.Commands() {
		if c.Name() == "serve" {
			found = true
			break
		}
	}
	if !found {
		t.Error("serve subcommand not found under mcp")
	}
}

func TestMCPServeCmd_Flags(t *testing.T) {
	root := cli.NewRootCmd("test")

	var mcpCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "mcp" {
			mcpCmd = c
			break
		}
	}
	if mcpCmd == nil {
		t.Fatal("mcp command not found")
	}

	var serveCmd *cobra.Command
	for _, c := range mcpCmd.Commands() {
		if c.Name() == "serve" {
			serveCmd = c
			break
		}
	}
	if serveCmd == nil {
		t.Fatal("serve command not found")
	}

	hostFlag := serveCmd.Flags().Lookup("host")
	if hostFlag == nil {
		t.Fatal("--host flag not found")
	}
	if hostFlag.DefValue != "127.0.0.1" {
		t.Errorf("host default = %q, want %q", hostFlag.DefValue, "127.0.0.1")
	}

	portFlag := serveCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatal("--port flag not found")
	}
	if portFlag.DefValue != "3100" {
		t.Errorf("port default = %q, want %q", portFlag.DefValue, "3100")
	}
}

func TestMCPServeCmd_DefaultValues(t *testing.T) {
	root := cli.NewRootCmd("test")

	var mcpCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "mcp" {
			mcpCmd = c
			break
		}
	}
	if mcpCmd == nil {
		t.Fatal("mcp command not found")
	}

	var serveCmd *cobra.Command
	for _, c := range mcpCmd.Commands() {
		if c.Name() == "serve" {
			serveCmd = c
			break
		}
	}
	if serveCmd == nil {
		t.Fatal("serve command not found")
	}

	hostFlag := serveCmd.Flags().Lookup("host")
	if hostFlag == nil {
		t.Fatal("--host flag not found")
	}
	if hostFlag.DefValue != "127.0.0.1" {
		t.Errorf("host default = %q, want 127.0.0.1", hostFlag.DefValue)
	}

	portFlag := serveCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatal("--port flag not found")
	}
	if portFlag.DefValue != "3100" {
		t.Errorf("port default = %q, want 3100", portFlag.DefValue)
	}
}
