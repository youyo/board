package config

import "fmt"

// GetCurrentProfile は cfg.CurrentProfile に対応する ProfileConfig を返す。
// CurrentProfile が空文字の場合は ErrInvalidConfig を返す。
// 対応するプロファイルが存在しない場合は ErrProfileNotFound を返す。
func GetCurrentProfile(cfg Config) (ProfileConfig, error) {
	if cfg.CurrentProfile == "" {
		return ProfileConfig{}, fmt.Errorf("%w: current_profile is empty", ErrInvalidConfig)
	}

	p, ok := cfg.Profiles[cfg.CurrentProfile]
	if !ok {
		return ProfileConfig{}, fmt.Errorf("%w: %q", ErrProfileNotFound, cfg.CurrentProfile)
	}

	return p, nil
}

// SetCurrentProfile は cfg.CurrentProfile を name に更新する。
func SetCurrentProfile(cfg *Config, name string) {
	cfg.CurrentProfile = name
}

// AddOrUpdateProfile はプロファイルを追加または更新する。
func AddOrUpdateProfile(cfg *Config, name string, profile ProfileConfig) {
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ProfileConfig)
	}
	cfg.Profiles[name] = profile
}

// ApplyDefaults はゼロ値フィールドにデフォルト値を適用した ProfileConfig を返す。
// TOMLパーサーはゼロ値と「未指定」を区別できないため、
// ロード後にこの関数を呼び出してデフォルト値を補完する。
//
// 注意: DailyAutoRefresh のデフォルトは true だが、
// TOML ゼロ値は false であるため、このフィールドは常に true に補完される。
// 明示的に false を設定したい場合はこの関数を呼び出さないこと。
func ApplyDefaults(p ProfileConfig) ProfileConfig {
	defaults := DefaultProfileConfig()

	if p.BaseURL == "" {
		p.BaseURL = defaults.BaseURL
	}
	if p.RequestTimeoutSeconds == 0 {
		p.RequestTimeoutSeconds = defaults.RequestTimeoutSeconds
	}
	if p.RetryMax == 0 {
		p.RetryMax = defaults.RetryMax
	}
	// DailyAutoRefresh: ゼロ値(false)はデフォルト(true)に補完
	// 将来的には *bool ポインタ型への移行を検討
	if !p.DailyAutoRefresh {
		p.DailyAutoRefresh = defaults.DailyAutoRefresh
	}

	return p
}
