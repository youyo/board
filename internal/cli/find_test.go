package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

func TestNewFindCmd(t *testing.T) {
	cmd := cli.NewFindCmd()

	if cmd.Use != "find" {
		t.Errorf("Use = %q, want %q", cmd.Use, "find")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	for _, name := range []string{"client", "project", "estimate", "invoice", "order", "delivery", "receipt"} {
		if !subNames[name] {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

func TestNewFindClientCmd(t *testing.T) {
	cmd := cli.NewFindClientCmd()
	if cmd.Use != "client" {
		t.Errorf("Use = %q, want %q", cmd.Use, "client")
	}

	for _, flagName := range []string{"id", "name", "text"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestNewFindProjectCmd(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	if cmd.Use != "project" {
		t.Errorf("Use = %q, want %q", cmd.Use, "project")
	}

	for _, flagName := range []string{"id", "client-name", "name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestRootCmdHasFindSubcommand(t *testing.T) {
	root := cli.NewRootCmd("test")
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "find" {
			found = true
		}
	}
	if !found {
		t.Error("root command does not have find subcommand")
	}
}

func TestFindClientCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindClientCmd()
	// Attach to a root so persistent flags work
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	// Execute without flags — should error
	root.SetArgs([]string{"client"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestFindProjectCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"project"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindEstimateCmd(t *testing.T) {
	cmd := cli.NewFindEstimateCmd()
	if cmd.Use != "estimate" {
		t.Errorf("Use = %q, want %q", cmd.Use, "estimate")
	}

	for _, flagName := range []string{"id", "client-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindEstimateCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindEstimateCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"estimate"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindInvoiceCmd(t *testing.T) {
	cmd := cli.NewFindInvoiceCmd()
	if cmd.Use != "invoice" {
		t.Errorf("Use = %q, want %q", cmd.Use, "invoice")
	}

	for _, flagName := range []string{"id", "client-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindInvoiceCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindInvoiceCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"invoice"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindOrderCmd(t *testing.T) {
	cmd := cli.NewFindOrderCmd()
	if cmd.Use != "order" {
		t.Errorf("Use = %q, want %q", cmd.Use, "order")
	}

	for _, flagName := range []string{"id", "client-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindOrderCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindOrderCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"order"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindDeliveryCmd(t *testing.T) {
	cmd := cli.NewFindDeliveryCmd()
	if cmd.Use != "delivery" {
		t.Errorf("Use = %q, want %q", cmd.Use, "delivery")
	}

	for _, flagName := range []string{"id", "client-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindDeliveryCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindDeliveryCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"delivery"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindReceiptCmd(t *testing.T) {
	cmd := cli.NewFindReceiptCmd()
	if cmd.Use != "receipt" {
		t.Errorf("Use = %q, want %q", cmd.Use, "receipt")
	}

	for _, flagName := range []string{"id", "client-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindReceiptCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindReceiptCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"receipt"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}
