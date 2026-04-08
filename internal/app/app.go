package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/config"
	"github.com/youyo/board/internal/refresh"
)

// App は board CLI の DI コンテナ。
// New() で初期化し、Close() で DB 接続を閉じる。
type App struct {
	Config  config.Config
	Profile config.ProfileConfig

	DB *cache.DB

	// cache ストア
	ResourceCache *cache.ResourceCache
	SyncStore     *cache.SyncStateStore

	// refresh エンジン
	Refresher   *refresh.Refresher
	LockManager *refresh.LockManager

	// boardapi クライアント
	APIClient *boardapi.Client

	// 全22リソース Repository
	Repos *Repositories
}

// New は App を初期化して返す。
// profileName が空の場合は config.CurrentProfile を使用する。
func New(profileName string) (*App, error) {
	// 1. config をロード
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return nil, fmt.Errorf("app: load config: %w", err)
	}

	// 2. プロファイルを解決
	if profileName == "" {
		profileName = cfg.CurrentProfile
	}
	prof, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("app: profile %q not found in config", profileName)
	}
	prof = config.ApplyDefaults(prof)

	// 3. DB パスを解決してオープン
	dp := dbPath()
	if err := os.MkdirAll(filepath.Dir(dp), 0o700); err != nil {
		return nil, fmt.Errorf("app: mkdir db dir: %w", err)
	}
	db, err := cache.Open(dp)
	if err != nil {
		return nil, fmt.Errorf("app: open db: %w", err)
	}

	// 4. マイグレーション
	if err := cache.Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("app: migrate: %w", err)
	}

	// 5. cache ストアを初期化
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)

	// 6. refresh エンジンを初期化
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "")

	// 7. タイムゾーンを解決（失敗時は UTC フォールバック）
	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		tz = time.UTC
	}

	// 8. boardapi クライアントを初期化
	timeout := time.Duration(prof.RequestTimeoutSeconds) * time.Second
	apiClient := boardapi.New(
		prof.BaseURL,
		prof.APIKey,
		prof.APIToken,
		timeout,
		boardapi.WithRetryMax(prof.RetryMax),
	)

	// 9. 全22リポジトリを初期化
	repos := newRepositories(profileName, apiClient, rc, ss, refresher, lm, tz, prof.DailyAutoRefresh)

	return &App{
		Config:        cfg,
		Profile:       prof,
		DB:            db,
		ResourceCache: rc,
		SyncStore:     ss,
		Refresher:     refresher,
		LockManager:   lm,
		APIClient:     apiClient,
		Repos:         repos,
	}, nil
}

// Close は DB 接続を閉じる。
func (a *App) Close() error {
	return a.DB.Close()
}

// dbPath は SQLite DB のファイルパスを返す。
//
// 優先順位:
//  1. BOARD_CACHE_PATH 環境変数
//  2. XDG_DATA_HOME/board/cache.db
//  3. HOME/.local/share/board/cache.db
//  4. os.UserCacheDir()/board/cache.db
//  5. $TMPDIR/board/cache.db（フォールバック）
func dbPath() string {
	if p := os.Getenv("BOARD_CACHE_PATH"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "board", "cache.db")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "board", "cache.db")
	}
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, "board", "cache.db")
	}
	return filepath.Join(os.TempDir(), "board", "cache.db")
}
