package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Load reads config.toml from the given path and returns a Config.
// If the file does not exist, it returns DefaultConfig() without an error.
// If the file exists but contains invalid TOML, it returns ErrInvalidConfig.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	// Correct for go-toml/v2 possibly setting empty sections to nil.
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ProfileConfig)
	}

	return cfg, nil
}

// Save saves Config to the given path in TOML format.
// Parent directories are created automatically if they do not exist.
// The file is created with 0600 permissions.
func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("%w: %w", ErrSaveConfig, err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSaveConfig, err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("%w: %w", ErrSaveConfig, err)
	}

	return nil
}
