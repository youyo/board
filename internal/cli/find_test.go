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

	for _, name := range []string{"client", "project", "estimate", "invoice", "order", "delivery", "receipt", "vendor", "purchase-order", "payment", "user", "group"} {
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

func TestNewFindVendorCmd(t *testing.T) {
	cmd := cli.NewFindVendorCmd()
	if cmd.Use != "vendor" {
		t.Errorf("Use = %q, want %q", cmd.Use, "vendor")
	}

	for _, flagName := range []string{"id", "name", "text"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindVendorCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindVendorCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"vendor"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindPurchaseOrderCmd(t *testing.T) {
	cmd := cli.NewFindPurchaseOrderCmd()
	if cmd.Use != "purchase-order" {
		t.Errorf("Use = %q, want %q", cmd.Use, "purchase-order")
	}

	for _, flagName := range []string{"id", "vendor-name", "project-name", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindPurchaseOrderCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindPurchaseOrderCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"purchase-order"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindPaymentCmd(t *testing.T) {
	cmd := cli.NewFindPaymentCmd()
	if cmd.Use != "payment" {
		t.Errorf("Use = %q, want %q", cmd.Use, "payment")
	}

	for _, flagName := range []string{"id", "vendor-name", "purchase-order-id", "text", "status"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindPaymentCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindPaymentCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"payment"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindUserCmd(t *testing.T) {
	cmd := cli.NewFindUserCmd()
	if cmd.Use != "user" {
		t.Errorf("Use = %q, want %q", cmd.Use, "user")
	}

	for _, flagName := range []string{"id", "name", "text"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindUserCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindUserCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"user"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}

func TestNewFindGroupCmd(t *testing.T) {
	cmd := cli.NewFindGroupCmd()
	if cmd.Use != "group" {
		t.Errorf("Use = %q, want %q", cmd.Use, "group")
	}

	for _, flagName := range []string{"id", "name", "text"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("--%s flag not defined", flagName)
		}
	}
}

func TestFindGroupCmdRequiresAtLeastOneFlag(t *testing.T) {
	cmd := cli.NewFindGroupCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)

	root.SetArgs([]string{"group"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no flags provided, got nil")
	}
}
