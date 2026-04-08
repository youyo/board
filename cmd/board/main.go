package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/cli"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "board",
		Short: "BOARD CLI - BOARD API client and MCP server",
		Long:  "board is a CLI tool for interacting with the BOARD API and running a local MCP server.",
	}

	rootCmd.Version = version
	rootCmd.SetVersionTemplate("board version {{.Version}}\n")

	rootCmd.AddCommand(cli.NewConfigureCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
