package cli_test

import (
	"strings"
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

	for _, name := range []string{"client", "project", "estimate", "invoice", "order", "delivery", "receipt", "vendor", "purchase-order", "payment", "user"} {
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

	for _, flagName := range []string{"id", "name"} {
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

	for _, flagName := range []string{"id", "client-name", "name", "status"} {
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

	for _, flagName := range []string{"id", "project-id", "client-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "client-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "project-id", "client-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "project-id", "client-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "project-id", "client-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "name"} {
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

	for _, flagName := range []string{"id", "vendor-name", "project-name", "status"} {
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

	for _, flagName := range []string{"id", "vendor-name", "purchase-order-id", "status"} {
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

	for _, flagName := range []string{"id", "name"} {
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

// TestNewFindGroupCmd / TestFindGroupCmdRequiresAtLeastOneFlag are removed in N07b
// because the find_group command is removed (ADR-001 Group 削除確定、find2 では FindGroup なし)。

// ========== CLI-T01〜T05: --statuses / --contract-status フラグ ==========

// CLI-T01: find project コマンドに --statuses フラグが定義されていること。
func TestFindProjectCmd_HasStatusesFlag(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	if f := cmd.Flags().Lookup("statuses"); f == nil {
		t.Error("--statuses flag not defined")
	}
}

// CLI-T02: find project コマンドに --contract-status フラグが定義されていること。
func TestFindProjectCmd_HasContractStatusFlag(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	if f := cmd.Flags().Lookup("contract-status"); f == nil {
		t.Error("--contract-status flag not defined")
	}
}

// CLI-T03: --statuses=受注済 で no-flag バリデーションを通過すること（フラグが「at least one of」に加算される）。
// (app context がないため実際のサービス呼び出し前に errNoApp が返る)
func TestFindProjectCmd_StatusesFlagCountsAsNarrow(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)
	root.SetArgs([]string{"project", "--statuses", "受注済"})
	err := root.Execute()
	// errNoApp か service エラーが期待される（"at least one of" ではない）
	if err == nil {
		t.Fatal("expected error (no app context), got nil")
	}
	if strings.Contains(err.Error(), "at least one of") {
		t.Errorf("--statuses should count as a narrowing flag, but got: %v", err)
	}
}

// CLI-T04: --contract-status=active で no-flag バリデーションを通過すること。
func TestFindProjectCmd_ContractStatusFlagCountsAsNarrow(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)
	root.SetArgs([]string{"project", "--contract-status", "active"})
	err := root.Execute()
	// errNoApp か service エラーが期待される（"at least one of" ではない）
	if err == nil {
		t.Fatal("expected error (no app context), got nil")
	}
	if strings.Contains(err.Error(), "at least one of") {
		t.Errorf("--contract-status should count as a narrowing flag, but got: %v", err)
	}
}

// CLI-T05: --status=受注済 --contract-status=active で 3-way 排他エラーが返ること（CLI 側チェック）。
func TestFindProjectCmd_StatusAndContractStatusAreMutuallyExclusive(t *testing.T) {
	cmd := cli.NewFindProjectCmd()
	root := &cobra.Command{Use: "board"}
	root.AddCommand(cmd)
	root.SetArgs([]string{"project", "--name", "保守", "--status", "受注済", "--contract-status", "active"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected mutually exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// ========== N07c: 構造的未対応 / 将来拡張 reject フラグの最終エラー文言確認 ==========

// rejectCases は CLI 側で resolver 経路に到達する前に reject されるべきフラグ組み合わせを確認する。
// 各 case は flag を 1 つだけ追加して実行し、エラーに含まれるべき substring を検証する。
// resolver 経路（--client-name / --vendor-name）は app context 経由で実行される必要があり
// 単体 cmd.Execute() ではテストできないため、ここでは「reject だけで完結する」フラグのみ扱う。
func TestFindCmd_RejectFlagErrorMessages(t *testing.T) {
	cases := []struct {
		name    string
		newCmd  func() *cobra.Command
		args    []string
		wantSub string
	}{
		{
			"estimate --id+--status",
			cli.NewFindEstimateCmd,
			[]string{"estimate", "--id", "1", "--status", "draft"},
			"--status filtering is not supported for documents",
		},
		{
			"order --id+--status",
			cli.NewFindOrderCmd,
			[]string{"order", "--id", "1", "--status", "x"},
			"--status filtering is not supported for documents",
		},
		{
			"delivery --id+--status",
			cli.NewFindDeliveryCmd,
			[]string{"delivery", "--id", "1", "--status", "x"},
			"--status filtering is not supported for documents",
		},
		{
			"receipt --id+--status",
			cli.NewFindReceiptCmd,
			[]string{"receipt", "--id", "1", "--status", "x"},
			"--status filtering is not supported for documents",
		},
		{
			"invoice --id+--project-name",
			cli.NewFindInvoiceCmd,
			[]string{"invoice", "--id", "1", "--project-name", "x"},
			"--project-name is not yet supported for invoices",
		},
		{
			"purchase-order --id+--project-name",
			cli.NewFindPurchaseOrderCmd,
			[]string{"purchase-order", "--id", "1", "--project-name", "x"},
			"--project-name is not yet supported for purchase orders",
		},
		{
			"payment --id+--purchase-order-id",
			cli.NewFindPaymentCmd,
			[]string{"payment", "--id", "1", "--purchase-order-id", "1"},
			"--purchase-order-id is not yet supported",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCmd()
			root := &cobra.Command{Use: "board"}
			root.AddCommand(cmd)
			root.SetArgs(tc.args)
			err := root.Execute()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
