package repository

// ReadOptions はリポジトリ読み取り時の動作を制御するオプション。
type ReadOptions struct {
	// Refresh が true の場合、差分リフレッシュ（DeltaRefresh）を実行する。
	Refresh bool
	// ForceRefresh が true の場合、全件リフレッシュ（ForceRefresh）を実行する。
	// Refresh より優先される。
	ForceRefresh bool
	// Limit は返却するエントリの最大件数。0 は無制限。
	Limit int
}
