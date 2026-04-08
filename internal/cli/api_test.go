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

	expects := []string{"clients", "client_branches", "contacts", "projects", "project_costs"}
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
