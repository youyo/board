package cli

import (
	"github.com/spf13/cobra"
)

// NewCompletionCmd はシェル補完スクリプト生成コマンドを返す。
func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "シェル補完スクリプトを生成",
	}
	cmd.AddCommand(newCompletionZshCmd(), newCompletionBashCmd())
	return cmd
}

func newCompletionZshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "zsh 用補完スクリプトを生成",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
}

func newCompletionBashCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "bash 用補完スクリプトを生成",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		},
	}
}
