package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureUseCmd は configure use コマンドを返す。
func NewConfigureUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use PROFILE",
		Short: "使用するプロファイルを切り替える",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			if _, ok := cfg.Profiles[profileName]; !ok {
				return fmt.Errorf("%w: %q", config.ErrProfileNotFound, profileName)
			}

			config.SetCurrentProfile(&cfg, profileName)
			return config.Save(cfg, path)
		},
	}
}
