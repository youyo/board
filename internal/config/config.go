// Package config provides configuration file management for the board CLI.
// It handles type definitions, default values, and profile management for config.toml.
package config

// Config is the top-level structure for config.toml.
type Config struct {
	CurrentProfile string                   `toml:"current_profile"`
	Timezone       string                   `toml:"timezone"`
	Profiles       map[string]ProfileConfig `toml:"profiles"`
}

// ProfileConfig holds per-profile settings.
type ProfileConfig struct {
	BaseURL               string `toml:"base_url"`
	APIKey                string `toml:"api_key"`
	APIToken              string `toml:"api_token"`
	DailyAutoRefresh      bool   `toml:"daily_auto_refresh"`
	RequestTimeoutSeconds int    `toml:"request_timeout_seconds"`
	RetryMax              int    `toml:"retry_max"`
	PrettyDefault         bool   `toml:"pretty_default"`
}

// String returns a string representation of ProfileConfig.
// Secret fields are masked to prevent APIKey/APIToken from being printed in plain text
// by fmt.Println and similar functions.
func (p ProfileConfig) String() string {
	return "ProfileConfig{...secrets masked...}"
}

// GoString returns a Go-syntax representation of ProfileConfig.
// It prevents secret information from leaking via the %#v format verb.
func (p ProfileConfig) GoString() string {
	return "config.ProfileConfig{...secrets masked...}"
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() Config {
	return Config{
		CurrentProfile: "default",
		Timezone:       "UTC",
		Profiles: map[string]ProfileConfig{
			"default": DefaultProfileConfig(),
		},
	}
}

// DefaultProfileConfig returns a ProfileConfig populated with default values.
func DefaultProfileConfig() ProfileConfig {
	return ProfileConfig{
		BaseURL:               "https://api.the-board.jp",
		DailyAutoRefresh:      true,
		RequestTimeoutSeconds: 30,
		RetryMax:              5,
		PrettyDefault:         false,
	}
}
