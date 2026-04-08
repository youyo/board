package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureGetCmd は configure get コマンドを返す。
func NewConfigureGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "設定値を取得する（secrets はマスク）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			path := config.ConfigPath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}

			val, isSecret, err := getField(cfg, key)
			if err != nil {
				return err
			}

			if isSecret {
				val = maskSecret(val)
			}

			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

// ParseKey は parseKey のエクスポート版（テスト用）。
func ParseKey(key string) (scope string, profileName string, field string, err error) {
	return parseKey(key)
}

// parseKey は KEY パス文字列を解析して scope, profileName, field を返す。
//
// 仕様:
//   - "timezone" → scope="top", profileName="", field="timezone"
//   - "current_profile" → scope="top", profileName="", field="current_profile"
//   - "profiles.<name>.<field>" → scope="profiles", profileName=<name>, field=<field>
//   - 上記以外 → ErrInvalidKey
func parseKey(key string) (scope string, profileName string, field string, err error) {
	parts := strings.SplitN(key, ".", 3)
	switch parts[0] {
	case "timezone", "current_profile":
		if len(parts) != 1 {
			return "", "", "", fmt.Errorf("invalid key: %q", key)
		}
		return "top", "", parts[0], nil
	case "profiles":
		if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
			return "", "", "", fmt.Errorf("invalid key: %q", key)
		}
		return "profiles", parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid key: %q", key)
	}
}

// validProfileFields は ProfileConfig の有効フィールド名セット
var validProfileFields = map[string]bool{
	"base_url":                true,
	"api_key":                 true,
	"api_token":               true,
	"daily_auto_refresh":      true,
	"request_timeout_seconds": true,
	"retry_max":               true,
	"pretty_default":          true,
}

// secretProfileFields はシークレットとして扱うフィールド名セット
var secretProfileFields = map[string]bool{
	"api_key":   true,
	"api_token": true,
}

// getField は Config から KEY パスに対応する値を文字列で返す。
// isSecret が true の場合はシークレットフィールド。
func getField(cfg config.Config, key string) (val string, isSecret bool, err error) {
	scope, profileName, field, err := parseKey(key)
	if err != nil {
		return "", false, err
	}

	switch scope {
	case "top":
		switch field {
		case "timezone":
			return cfg.Timezone, false, nil
		case "current_profile":
			return cfg.CurrentProfile, false, nil
		}
	case "profiles":
		if !validProfileFields[field] {
			return "", false, fmt.Errorf("invalid key: %q", key)
		}
		prof, ok := cfg.Profiles[profileName]
		if !ok {
			return "", false, fmt.Errorf("%w: %q", config.ErrProfileNotFound, profileName)
		}
		isSecret = secretProfileFields[field]
		switch field {
		case "base_url":
			return prof.BaseURL, isSecret, nil
		case "api_key":
			return prof.APIKey, isSecret, nil
		case "api_token":
			return prof.APIToken, isSecret, nil
		case "daily_auto_refresh":
			return strconv.FormatBool(prof.DailyAutoRefresh), isSecret, nil
		case "request_timeout_seconds":
			return strconv.Itoa(prof.RequestTimeoutSeconds), isSecret, nil
		case "retry_max":
			return strconv.Itoa(prof.RetryMax), isSecret, nil
		case "pretty_default":
			return strconv.FormatBool(prof.PrettyDefault), isSecret, nil
		}
	}
	return "", false, fmt.Errorf("invalid key: %q", key)
}
