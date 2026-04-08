package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureShowCmd は configure show コマンドを返す。
func NewConfigureShowCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "現在のプロファイル設定を表示する（secrets はマスク）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			profileName := profileFlag
			if profileName == "" {
				profileName = cfg.CurrentProfile
			}

			prof, ok := cfg.Profiles[profileName]
			if !ok {
				return fmt.Errorf("%w: %q", config.ErrProfileNotFound, profileName)
			}

			out := map[string]interface{}{
				"profile": profileName,
				"config": map[string]interface{}{
					"base_url":                prof.BaseURL,
					"api_key":                 maskSecret(prof.APIKey),
					"api_token":               maskSecret(prof.APIToken),
					"daily_auto_refresh":      prof.DailyAutoRefresh,
					"request_timeout_seconds": prof.RequestTimeoutSeconds,
					"retry_max":               prof.RetryMax,
					"pretty_default":          prof.PrettyDefault,
				},
				"global": map[string]interface{}{
					"current_profile": cfg.CurrentProfile,
					"timezone":        cfg.Timezone,
				},
			}

			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	cmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "表示するプロファイル名")
	return cmd
}
