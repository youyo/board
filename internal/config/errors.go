package config

import "errors"

var (
	// ErrInvalidConfig is returned when the configuration file is invalid.
	ErrInvalidConfig = errors.New("invalid config")
	// ErrSaveConfig is returned when saving the configuration file fails.
	ErrSaveConfig = errors.New("failed to save config")
	// ErrProfileNotFound is returned when the specified profile does not exist.
	ErrProfileNotFound = errors.New("profile not found")
)
