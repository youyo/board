package cli

import (
	"github.com/spf13/cobra"
)

// NewCompletionCmd returns the shell completion script generation command.
func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
	}
	cmd.AddCommand(newCompletionZshCmd(), newCompletionBashCmd())
	return cmd
}

func newCompletionZshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate completion script for zsh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
}

func newCompletionBashCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate completion script for bash",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		},
	}
}
