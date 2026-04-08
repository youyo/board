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

// String は ProfileConfig の文字列表現を返す。
// fmt.Println 等でAPIKey/APITokenが平文出力されることを防ぐため、
// 秘密情報はマスクする。
func (p ProfileConfig) String() string {
	return "ProfileConfig{...secrets masked...}"
}

// GoString は ProfileConfig の Go 構文表現を返す。
// %#v フォーマットでの秘密情報漏洩を防ぐ。
func (p ProfileConfig) GoString() string {
	return "config.ProfileConfig{...secrets masked...}"
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
