package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureGetCmd returns the configure get command.
func NewConfigureGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Get a configuration value (secrets are masked)",
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

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

// ParseKey is the exported version of parseKey (for testing).
func ParseKey(key string) (scope string, profileName string, field string, err error) {
	return parseKey(key)
}

// parseKey parses a KEY path string and returns scope, profileName, and field.
//
// Format:
//   - "timezone" → scope="top", profileName="", field="timezone"
//   - "current_profile" → scope="top", profileName="", field="current_profile"
//   - "profiles.<name>.<field>" → scope="profiles", profileName=<name>, field=<field>
//   - anything else → ErrInvalidKey
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

// validProfileFields is the set of valid field names for ProfileConfig.
var validProfileFields = map[string]bool{
	"base_url":                true,
	"api_key":                 true,
	"api_token":               true,
	"request_timeout_seconds": true,
	"retry_max":               true,
	"pretty_default":          true,
}

// secretProfileFields is the set of field names treated as secrets.
var secretProfileFields = map[string]bool{
	"api_key":   true,
	"api_token": true,
}

// getField returns the string value corresponding to the KEY path in Config.
// isSecret is true when the field is a secret.
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
