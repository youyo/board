package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
)

// globalFlags holds the values of common flags received by the root command.
type globalFlags struct {
	profile      string
	refresh      bool
	forceRefresh bool
	pretty       bool
	limit        int
}

// NewRootCmd returns the root Cobra command for board.
// version is the embedded version string.
func NewRootCmd(version string) *cobra.Command {
	var gf globalFlags

	rootCmd := &cobra.Command{
		Use:   "board",
		Short: "BOARD CLI - BOARD API client and MCP server",
		Long:  "board is a CLI client and local HTTP MCP server for the BOARD API.",
		// PersistentPreRunE skips App initialization for configure commands and initializes App for all others.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// configure commands do not require App initialization (they do not depend on the config file).
			if strings.HasPrefix(cmd.CommandPath(), "board configure") {
				return nil
			}

			a, err := app.New(gf.profile)
			if err != nil {
				return err
			}

			// Store App in context so subcommands can retrieve it via AppFromContext.
			ctx := app.WithApp(cmd.Context(), a)
			cmd.SetContext(ctx)
			return nil
		},
		// PersistentPostRunE closes the DB connection. It also runs when RunE returns an error.
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// configure commands do not initialize App, so skip closing.
			if strings.HasPrefix(cmd.CommandPath(), "board configure") {
				return nil
			}

			a, ok := app.AppFromContext(cmd.Context())
			if !ok {
				return nil
			}
			return a.Close()
		},
	}

	rootCmd.Version = version
	rootCmd.SetVersionTemplate("board version {{.Version}}\n")
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	// Common flags (inherited by all subcommands).
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&gf.profile, "profile", "p", "", "Profile name to use (default: current_profile)")
	pf.BoolVar(&gf.refresh, "refresh", false, "Force incremental refresh")
	pf.BoolVar(&gf.forceRefresh, "force-refresh", false, "Force full refresh")
	pf.BoolVar(&gf.pretty, "pretty", false, "Pretty-print JSON output")
	pf.IntVar(&gf.limit, "limit", 0, "Maximum number of results to return (0 = unlimited)")

	// Register subcommands.
	rootCmd.AddCommand(NewConfigureCmd())
	rootCmd.AddCommand(NewAPICmd())
	rootCmd.AddCommand(NewFindCmd())
	rootCmd.AddCommand(NewCacheCmd())
	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(NewDocsCmd())
	rootCmd.AddCommand(NewMCPCmd())

	return rootCmd
}
