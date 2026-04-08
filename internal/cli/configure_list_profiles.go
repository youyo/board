package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureListProfilesCmd は configure list-profiles コマンドを返す。
func NewConfigureListProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-profiles",
		Short: "プロファイル一覧を表示する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
}
