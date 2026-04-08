package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

// TestNewAPICmd はコマンド構造（サブコマンド・フラグ）を検証する。
func TestNewAPICmd(t *testing.T) {
	cmd := cli.NewAPICmd()

	if cmd.Use != "api" {
		t.Errorf("Use = %q, want %q", cmd.Use, "api")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	expects := []string{"clients", "client_branches", "contacts", "projects", "project_costs", "estimates", "invoices", "orders", "deliveries", "receipts", "vendors", "vendor_branches", "vendor_contacts", "purchase_orders", "payments", "users", "groups", "payment_terms", "project_types", "purchase_types", "accounting_types", "document_send_channels"}
	for _, name := range expects {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}
}

func TestNewAPIClientsCmd(t *testing.T) {
	cmd := cli.NewAPIClientsCmd()
	if cmd.Use != "clients" {
		t.Errorf("Use = %q, want %q", cmd.Use, "clients")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	// get コマンドに --id フラグがあるか確認
	var getCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "get" {
			getCmd = sub
		}
	}
	if getCmd == nil {
		t.Fatal("get コマンドが見つからない")
	}
	if f := getCmd.Flags().Lookup("id"); f == nil {
		t.Error("get: --id フラグが定義されていない")
	}

	// search コマンドのフラグ確認
	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"name", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIClientBranchesCmd(t *testing.T) {
	cmd := cli.NewAPIClientBranchesCmd()
	if cmd.Use != "client_branches" {
		t.Errorf("Use = %q, want %q", cmd.Use, "client_branches")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	// search --client-id, --name
	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"client-id", "name"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIContactsCmd(t *testing.T) {
	cmd := cli.NewAPIContactsCmd()
	if cmd.Use != "contacts" {
		t.Errorf("Use = %q, want %q", cmd.Use, "contacts")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"client-id", "name", "email"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIProjectsCmd(t *testing.T) {
	cmd := cli.NewAPIProjectsCmd()
	if cmd.Use != "projects" {
		t.Errorf("Use = %q, want %q", cmd.Use, "projects")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"client-id", "name", "status", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIProjectCostsCmd(t *testing.T) {
	cmd := cli.NewAPIProjectCostsCmd()
	if cmd.Use != "project_costs" {
		t.Errorf("Use = %q, want %q", cmd.Use, "project_costs")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	if f := searchCmd.Flags().Lookup("project-id"); f == nil {
		t.Error("search: --project-id フラグが定義されていない")
	}
}

func testDocumentCmd(t *testing.T, cmd *cobra.Command, use string, searchFlags []string) {
	t.Helper()
	if cmd.Use != use {
		t.Errorf("Use = %q, want %q", cmd.Use, use)
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	// get --id フラグ確認
	var getCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "get" {
			getCmd = sub
		}
	}
	if getCmd == nil {
		t.Fatal("get コマンドが見つからない")
	}
	if f := getCmd.Flags().Lookup("id"); f == nil {
		t.Error("get: --id フラグが定義されていない")
	}

	// search フラグ確認
	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range searchFlags {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIEstimatesCmd(t *testing.T) {
	testDocumentCmd(t, cli.NewAPIEstimatesCmd(), "estimates",
		[]string{"client-id", "project-id", "status", "updated-at-from"})
}

func TestNewAPIInvoicesCmd(t *testing.T) {
	testDocumentCmd(t, cli.NewAPIInvoicesCmd(), "invoices",
		[]string{"client-id", "project-id", "status", "updated-at-from"})
}

func TestNewAPIOrdersCmd(t *testing.T) {
	testDocumentCmd(t, cli.NewAPIOrdersCmd(), "orders",
		[]string{"client-id", "project-id", "status", "updated-at-from"})
}

func TestNewAPIDeliveriesCmd(t *testing.T) {
	testDocumentCmd(t, cli.NewAPIDeliveriesCmd(), "deliveries",
		[]string{"client-id", "project-id", "status", "updated-at-from"})
}

func TestNewAPIReceiptsCmd(t *testing.T) {
	testDocumentCmd(t, cli.NewAPIReceiptsCmd(), "receipts",
		[]string{"client-id", "project-id", "status", "updated-at-from"})
}

func TestNewAPIVendorsCmd(t *testing.T) {
	cmd := cli.NewAPIVendorsCmd()
	if cmd.Use != "vendors" {
		t.Errorf("Use = %q, want %q", cmd.Use, "vendors")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"name", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIVendorBranchesCmd(t *testing.T) {
	cmd := cli.NewAPIVendorBranchesCmd()
	if cmd.Use != "vendor_branches" {
		t.Errorf("Use = %q, want %q", cmd.Use, "vendor_branches")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"vendor-id", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIVendorContactsCmd(t *testing.T) {
	cmd := cli.NewAPIVendorContactsCmd()
	if cmd.Use != "vendor_contacts" {
		t.Errorf("Use = %q, want %q", cmd.Use, "vendor_contacts")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"vendor-id", "name", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIPurchaseOrdersCmd(t *testing.T) {
	cmd := cli.NewAPIPurchaseOrdersCmd()
	if cmd.Use != "purchase_orders" {
		t.Errorf("Use = %q, want %q", cmd.Use, "purchase_orders")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"vendor-id", "project-id", "status", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIPaymentsCmd(t *testing.T) {
	cmd := cli.NewAPIPaymentsCmd()
	if cmd.Use != "payments" {
		t.Errorf("Use = %q, want %q", cmd.Use, "payments")
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range []string{"vendor-id", "purchase-order-id", "status", "updated-at-from"} {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func testMasterCmd(t *testing.T, cmd *cobra.Command, use string, searchFlags []string) {
	t.Helper()
	if cmd.Use != use {
		t.Errorf("Use = %q, want %q", cmd.Use, use)
	}
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}
	for _, name := range []string{"list", "get", "search"} {
		if !subNames[name] {
			t.Errorf("サブコマンド %q が登録されていない", name)
		}
	}

	var getCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "get" {
			getCmd = sub
		}
	}
	if getCmd == nil {
		t.Fatal("get コマンドが見つからない")
	}
	if f := getCmd.Flags().Lookup("id"); f == nil {
		t.Error("get: --id フラグが定義されていない")
	}

	var searchCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "search" {
			searchCmd = sub
		}
	}
	if searchCmd == nil {
		t.Fatal("search コマンドが見つからない")
	}
	for _, flagName := range searchFlags {
		if f := searchCmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("search: --%s フラグが定義されていない", flagName)
		}
	}
}

func TestNewAPIUsersCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIUsersCmd(), "users",
		[]string{"name", "email", "updated-at-from"})
}

func TestNewAPIGroupsCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIGroupsCmd(), "groups",
		[]string{"name", "updated-at-from"})
}

func TestNewAPIPaymentTermsCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIPaymentTermsCmd(), "payment_terms",
		[]string{"name", "updated-at-from"})
}

func TestNewAPIProjectTypesCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIProjectTypesCmd(), "project_types",
		[]string{"name", "updated-at-from"})
}

func TestNewAPIPurchaseTypesCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIPurchaseTypesCmd(), "purchase_types",
		[]string{"name", "updated-at-from"})
}

func TestNewAPIAccountingTypesCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIAccountingTypesCmd(), "accounting_types",
		[]string{"name", "updated-at-from"})
}

func TestNewAPIDocumentSendChannelsCmd(t *testing.T) {
	testMasterCmd(t, cli.NewAPIDocumentSendChannelsCmd(), "document_send_channels",
		[]string{"name", "updated-at-from"})
}

func TestRootCmdHasAPISubcommand(t *testing.T) {
	root := cli.NewRootCmd("test")
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "api" {
			found = true
		}
	}
	if !found {
		t.Error("root コマンドに api サブコマンドが登録されていない")
	}
}
