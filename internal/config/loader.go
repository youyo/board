package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Load は指定パスの config.toml を読み込んで Config を返す。
// ファイルが存在しない場合は DefaultConfig() を返す（エラーなし）。
// ファイルが存在するが不正な TOML の場合は ErrInvalidConfig を返す。
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

	// go-toml/v2 は空セクションを nil にする場合があるため補正
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ProfileConfig)
	}

	return cfg, nil
}

// Save は Config を指定パスに TOML 形式で保存する。
// 親ディレクトリが存在しない場合は自動作成する。
// ファイルは 0600 パーミッションで作成する。
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
