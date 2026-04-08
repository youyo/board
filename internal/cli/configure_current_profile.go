package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureCurrentProfileCmd は configure current-profile コマンドを返す。
func NewConfigureCurrentProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-profile",
		Short: "現在のプロファイル名を表示する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), cfg.CurrentProfile)
			return nil
		},
	}
}
