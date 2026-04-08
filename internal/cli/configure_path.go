package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigurePathCmd は configure path コマンドを返す。
func NewConfigurePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "設定ファイルのパスを表示する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), config.ConfigPath())
			return nil
		},
	}
}
