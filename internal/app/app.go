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

// App is the DI container for the board CLI.
// Initialize with New() and close the DB connection with Close().
type App struct {
	Config      config.Config
	Profile     config.ProfileConfig
	ProfileName string

	DBPath string
	DB     *cache.DB

	// cache stores
	ResourceCache *cache.ResourceCache
	SyncStore     *cache.SyncStateStore

	// refresh engine
	Refresher   *refresh.Refresher
	LockManager *refresh.LockManager

	// boardapi client
	APIClient *boardapi.Client

	// Repositories for all 22 resources
	Repos *Repositories
}

// New initializes and returns an App.
// If profileName is empty, config.CurrentProfile is used.
func New(profileName string) (*App, error) {
	// 1. Load config
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return nil, fmt.Errorf("app: load config: %w", err)
	}

	// 2. Resolve profile
	if profileName == "" {
		profileName = cfg.CurrentProfile
	}
	prof, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("app: profile %q not found in config", profileName)
	}
	prof = config.ApplyDefaults(prof)
	prof = config.ApplyEnvOverrides(prof)

	// 3. Resolve DB path and open
	dp := dbPath()
	if err := os.MkdirAll(filepath.Dir(dp), 0o700); err != nil {
		return nil, fmt.Errorf("app: mkdir db dir: %w", err)
	}
	db, err := cache.Open(dp)
	if err != nil {
		return nil, fmt.Errorf("app: open db: %w", err)
	}

	// 4. Migrate
	if err := cache.Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("app: migrate: %w", err)
	}

	// 5. Initialize cache stores
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)

	// 6. Initialize refresh engine
	refresher := refresh.NewRefresher(rc, ss)
	lm := refresh.NewLockManager(ss, "")

	// 7. Resolve timezone (fallback to UTC on failure)
	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		tz = time.UTC
	}

	// 8. Initialize boardapi client
	timeout := time.Duration(prof.RequestTimeoutSeconds) * time.Second
	apiClient := boardapi.New(
		prof.BaseURL,
		prof.APIKey,
		prof.APIToken,
		timeout,
		boardapi.WithRetryMax(prof.RetryMax),
	)

	// 9. Initialize all 22 repositories
	repos := newRepositories(profileName, apiClient, rc, ss, refresher, lm, tz, prof.DailyAutoRefresh)

	return &App{
		Config:        cfg,
		Profile:       prof,
		ProfileName:   profileName,
		DBPath:        dp,
		DB:            db,
		ResourceCache: rc,
		SyncStore:     ss,
		Refresher:     refresher,
		LockManager:   lm,
		APIClient:     apiClient,
		Repos:         repos,
	}, nil
}

// Close closes the DB connection.
func (a *App) Close() error {
	return a.DB.Close()
}

// dbPath returns the file path for the SQLite DB.
//
// Resolution order:
//  1. BOARD_CACHE_PATH environment variable
//  2. XDG_CACHE_HOME/board/cache.db
//  3. HOME/.cache/board/cache.db
//  4. $TMPDIR/board/cache.db (fallback)
func dbPath() string {
	if p := os.Getenv("BOARD_CACHE_PATH"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "board", "cache.db")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "board", "cache.db")
	}
	return filepath.Join(os.TempDir(), "board", "cache.db")
}
