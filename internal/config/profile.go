package config

import (
	"fmt"
	"os"
)

// GetCurrentProfile returns the ProfileConfig corresponding to cfg.CurrentProfile.
// Returns ErrInvalidConfig if CurrentProfile is empty.
// Returns ErrProfileNotFound if the corresponding profile does not exist.
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

// SetCurrentProfile updates cfg.CurrentProfile to name.
func SetCurrentProfile(cfg *Config, name string) {
	cfg.CurrentProfile = name
}

// AddOrUpdateProfile adds or updates a profile.
func AddOrUpdateProfile(cfg *Config, name string, profile ProfileConfig) {
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ProfileConfig)
	}
	cfg.Profiles[name] = profile
}

// ApplyDefaults returns a ProfileConfig with default values applied to zero-value fields.
// Because the TOML parser cannot distinguish between a zero value and "not specified",
// call this function after loading to fill in default values.
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

	return p
}

// ApplyEnvOverrides overrides ProfileConfig fields with environment variable values.
// Only non-empty environment variable values are applied.
//
// Supported environment variables:
//   - BOARD_API_KEY: overrides APIKey
//   - BOARD_API_TOKEN: overrides APIToken
func ApplyEnvOverrides(p ProfileConfig) ProfileConfig {
	if v := os.Getenv("BOARD_API_KEY"); v != "" {
		p.APIKey = v
	}
	if v := os.Getenv("BOARD_API_TOKEN"); v != "" {
		p.APIToken = v
	}
	return p
}
