package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureCurrentProfileCmd returns the configure current-profile command.
func NewConfigureCurrentProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-profile",
		Short: "Show the current profile name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), cfg.CurrentProfile)
			return nil
		},
	}
}
