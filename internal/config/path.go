package config

import (
	"os"
	"path/filepath"
)

// ConfigPath は config.toml のファイルパスを返す。
// 優先順位:
//  1. BOARD_CONFIG_PATH 環境変数
//  2. XDG_CONFIG_HOME/board/config.toml
//  3. HOME/.config/board/config.toml
//  4. $TMPDIR/board/config.toml（フォールバック）
func ConfigPath() string {
	// 1. 環境変数による上書き
	if p := os.Getenv("BOARD_CONFIG_PATH"); p != "" {
		return p
	}

	// 2. XDG_CONFIG_HOME
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "board", "config.toml")
	}

	// 3. HOME/.config
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "board", "config.toml")
	}

	// 4. os.UserConfigDir() による標準パス
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "board", "config.toml")
	}

	// 5. フォールバック: TMPDIR
	return filepath.Join(os.TempDir(), "board", "config.toml")
}
