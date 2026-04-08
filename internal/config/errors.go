package config

import "errors"

var (
	// ErrInvalidConfig は設定ファイルが不正な場合のエラー
	ErrInvalidConfig = errors.New("invalid config")
	// ErrSaveConfig は設定ファイルの保存に失敗した場合のエラー
	ErrSaveConfig = errors.New("failed to save config")
	// ErrProfileNotFound は指定したプロファイルが存在しない場合のエラー
	ErrProfileNotFound = errors.New("profile not found")
)
