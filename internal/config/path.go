package config

import (
	"os"
	"path/filepath"
)

// ConfigPath returns the file path of config.toml.
// Resolution order:
//  1. BOARD_CONFIG_PATH environment variable
//  2. XDG_CONFIG_HOME/board/config.toml
//  3. HOME/.config/board/config.toml
//  4. $TMPDIR/board/config.toml (fallback)
func ConfigPath() string {
	// 1. Override via environment variable
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

	// 4. Standard path via os.UserConfigDir()
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "board", "config.toml")
	}

	// 5. Fallback: TMPDIR
	return filepath.Join(os.TempDir(), "board", "config.toml")
}
