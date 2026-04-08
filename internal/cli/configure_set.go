package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureSetCmd returns the configure set command.
func NewConfigureSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Update a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			if err := setField(&cfg, key, value); err != nil {
				return err
			}

			return config.Save(cfg, path)
		},
	}
}

// setField sets value at the KEY path in Config.
func setField(cfg *config.Config, key string, value string) error {
	scope, profileName, field, err := parseKey(key)
	if err != nil {
		return err
	}

	switch scope {
	case "top":
		switch field {
		case "timezone":
			cfg.Timezone = value
		case "current_profile":
			cfg.CurrentProfile = value
		}
		return nil
	case "profiles":
		if !validProfileFields[field] {
			return fmt.Errorf("invalid key: %q", key)
		}
		prof := cfg.Profiles[profileName] // zero value is acceptable
		switch field {
		case "base_url":
			prof.BaseURL = value
		case "api_key":
			prof.APIKey = value
		case "api_token":
			prof.APIToken = value
		case "daily_auto_refresh":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid value for %q: %w", key, err)
			}
			prof.DailyAutoRefresh = b
		case "request_timeout_seconds":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid value for %q: %w", key, err)
			}
			prof.RequestTimeoutSeconds = n
		case "retry_max":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid value for %q: %w", key, err)
			}
			prof.RetryMax = n
		case "pretty_default":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid value for %q: %w", key, err)
			}
			prof.PrettyDefault = b
		}
		config.AddOrUpdateProfile(cfg, profileName, prof)
		return nil
	}
	return fmt.Errorf("invalid key: %q", key)
}
