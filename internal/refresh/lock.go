package refresh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/youyo/board/internal/cache"
)

const staleLockTimeout = 10 * time.Minute

var (
	// ErrLockBusy は in-process mutex が取得できない場合に返す（将来の非ブロッキング対応用）。
	ErrLockBusy = errors.New("refresh lock is busy")
	// ErrLockCanceled は ctx がキャンセルされた場合に返す。
	ErrLockCanceled = errors.New("lock acquisition canceled")
)

// lockKey は profile+resource の組み合わせを表すマップキー。
type lockKey struct {
	Profile  string
	Resource string
}

// LockManager は profile×resource 単位の in-process mutex と
// DB ロック（sync_state）を管理する。
type LockManager struct {
	mu        sync.Mutex              // locks マップ自体の保護
	locks     map[lockKey]*sync.Mutex // per-(profile,resource) mutex
	syncStore *cache.SyncStateStore
	updater   *Updater
	ownerID   string           // このプロセスの識別子（hostname+PID）
	now       func() time.Time // テスト差し込み可能な時計
}

// defaultOwnerID は hostname+PID からプロセス識別子を生成する。
func defaultOwnerID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

// NewLockManager は LockManager を生成する。
// ownerID が空の場合、hostname+PID から自動生成する。
func NewLockManager(ss *cache.SyncStateStore, ownerID string) *LockManager {
	if ownerID == "" {
		ownerID = defaultOwnerID()
	}
	return &LockManager{
		locks:     make(map[lockKey]*sync.Mutex),
		syncStore: ss,
		updater:   NewUpdater(ss),
		ownerID:   ownerID,
		now:       time.Now,
	}
}

// getKeyMutex は指定 key の per-key mutex を取得する（なければ作成する）。
func (m *LockManager) getKeyMutex(key lockKey) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.locks[key]; !ok {
		m.locks[key] = &sync.Mutex{}
	}
	return m.locks[key]
}

// isStale は SyncState の RefreshStartedAt が staleLockTimeout を超えているか判定する。
// RefreshStartedAt が NULL または無効な場合は false を返す。
func (m *LockManager) isStale(state *cache.SyncState) bool {
	if state == nil || !state.RefreshStartedAt.Valid {
		return false
	}
	startedAt, err := time.Parse(time.RFC3339, state.RefreshStartedAt.String)
	if err != nil {
		// パース失敗は stale とみなさない（安全側）
		return false
	}
	return m.now().Sub(startedAt) > staleLockTimeout
}

// AcquireLock はロックを取得する。
//  1. ctx がキャンセル済みの場合は ErrLockCanceled を返す
//  2. per-key mutex を Lock（同一プロセス内の直列化）
//  3. DB の refresh_started_at を読み stale 判定（stale なら上書き可）
//  4. DB に refresh_started_at=now, refresh_owner=ownerID を書く
func (m *LockManager) AcquireLock(ctx context.Context, profile, resource string) error {
	// ctx キャンセルチェック（mutex 取得前）
	if ctx.Err() != nil {
		return ErrLockCanceled
	}

	key := lockKey{Profile: profile, Resource: resource}
	keyMu := m.getKeyMutex(key)
	keyMu.Lock()

	// mutex 取得後も ctx をチェック
	if ctx.Err() != nil {
		keyMu.Unlock()
		return ErrLockCanceled
	}

	// DB stale チェック（別プロセスのロック観測・復旧用）
	// in-process mutex が主制御なので、非 stale の DB ロックも MVP では続行する
	_, err := m.syncStore.Get(ctx, profile, resource)
	if err != nil {
		keyMu.Unlock()
		return err
	}

	// DB にロック情報を書き込む
	if err := m.updater.MarkLockAcquired(ctx, profile, resource, m.ownerID, m.now()); err != nil {
		keyMu.Unlock()
		return err
	}

	return nil
}

// ReleaseLock はロックを解放する。
//  1. DB の refresh_started_at, refresh_owner を NULL に戻す
//  2. per-key mutex を Unlock
func (m *LockManager) ReleaseLock(ctx context.Context, profile, resource string) error {
	key := lockKey{Profile: profile, Resource: resource}

	// DB のロック情報をクリア
	dbErr := m.updater.MarkLockReleased(ctx, profile, resource)

	// per-key mutex を解放（DB エラーがあっても解放する）
	m.mu.Lock()
	keyMu, ok := m.locks[key]
	m.mu.Unlock()
	if ok {
		keyMu.Unlock()
	}

	return dbErr
}

// WithLock はロック取得 → fn 実行 → ロック解放 を保証するヘルパー。
// fn が panic した場合でも defer で ReleaseLock を呼ぶ。
// ReleaseLock は ctx キャンセル後も確実に実行するため context.Background() を使用する。
func (m *LockManager) WithLock(ctx context.Context, profile, resource string, fn func() error) error {
	if err := m.AcquireLock(ctx, profile, resource); err != nil {
		return err
	}
	defer func() {
		_ = m.ReleaseLock(context.Background(), profile, resource)
	}()
	return fn()
}
