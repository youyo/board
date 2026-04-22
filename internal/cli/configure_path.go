package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigurePathCmd returns the configure path command.
func NewConfigurePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the path to the configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), config.ConfigPath())
			return nil
		},
	}
}
