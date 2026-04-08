// Package config は board CLI の設定ファイル管理を提供する。
// config.toml の型定義、デフォルト値、プロファイル管理を担う。
package config

// Config は config.toml のトップレベル構造体
type Config struct {
	CurrentProfile string                    `toml:"current_profile"`
	Timezone       string                    `toml:"timezone"`
	Profiles       map[string]ProfileConfig  `toml:"profiles"`
}

// ProfileConfig はプロファイルごとの設定
type ProfileConfig struct {
	BaseURL               string `toml:"base_url"`
	APIKey                string `toml:"api_key"`
	APIToken              string `toml:"api_token"`
	DailyAutoRefresh      bool   `toml:"daily_auto_refresh"`
	RequestTimeoutSeconds int    `toml:"request_timeout_seconds"`
	RetryMax              int    `toml:"retry_max"`
	PrettyDefault         bool   `toml:"pretty_default"`
}

// DefaultConfig はデフォルト値を持つ Config を返す
func DefaultConfig() Config {
	return Config{
		CurrentProfile: "default",
		Timezone:       "UTC",
		Profiles: map[string]ProfileConfig{
			"default": DefaultProfileConfig(),
		},
	}
}

// DefaultProfileConfig はデフォルト値を持つ ProfileConfig を返す
func DefaultProfileConfig() ProfileConfig {
	return ProfileConfig{
		BaseURL:               "https://api.the-board.jp",
		DailyAutoRefresh:      true,
		RequestTimeoutSeconds: 30,
		RetryMax:              5,
		PrettyDefault:         false,
	}
}
