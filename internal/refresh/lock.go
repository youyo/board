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
	// ErrLockBusy is returned when the in-process mutex cannot be acquired (for future non-blocking support).
	ErrLockBusy = errors.New("refresh lock is busy")
	// ErrLockCanceled is returned when ctx is cancelled.
	ErrLockCanceled = errors.New("lock acquisition canceled")
)

// RefreshInProgressError indicates that another in-flight refresh is holding the lock.
// Returned by TryAcquireLock / TryWithLock so callers can return 429-style responses.
type RefreshInProgressError struct {
	Profile           string
	Resource          string
	Holder            string
	ElapsedSeconds    int
	RetryAfterSeconds int
}

func (e *RefreshInProgressError) Error() string {
	return fmt.Sprintf("refresh in progress for %s (holder=%s, elapsed=%ds)", e.Resource, e.Holder, e.ElapsedSeconds)
}

// IsRefreshInProgress reports whether err is a RefreshInProgressError.
func IsRefreshInProgress(err error) bool {
	var rip *RefreshInProgressError
	return errors.As(err, &rip)
}

// lockKey is a map key representing a profile+resource combination.
type lockKey struct {
	Profile  string
	Resource string
}

// LockManager manages per (profile×resource) in-process mutexes
// and DB locks (sync_state).
type LockManager struct {
	mu        sync.Mutex              // protects the locks map itself
	locks     map[lockKey]*sync.Mutex // per-(profile,resource) mutex
	syncStore *cache.SyncStateStore
	updater   *Updater
	ownerID   string           // identifier for this process (hostname+PID)
	now       func() time.Time // injectable clock for testing
}

// defaultOwnerID generates a process identifier from hostname+PID.
func defaultOwnerID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

// NewLockManager creates a LockManager.
// If ownerID is empty, it is auto-generated from hostname+PID.
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

// getKeyMutex returns the per-key mutex for the specified key (creates it if not present).
func (m *LockManager) getKeyMutex(key lockKey) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.locks[key]; !ok {
		m.locks[key] = &sync.Mutex{}
	}
	return m.locks[key]
}

// isStale checks if SyncState.RefreshStartedAt has exceeded staleLockTimeout.
// Returns false if RefreshStartedAt is NULL or invalid.
func (m *LockManager) isStale(state *cache.SyncState) bool {
	if state == nil || !state.RefreshStartedAt.Valid {
		return false
	}
	startedAt, err := time.Parse(time.RFC3339, state.RefreshStartedAt.String)
	if err != nil {
		// parse failure is not treated as stale (safe side)
		return false
	}
	return m.now().Sub(startedAt) > staleLockTimeout
}

// AcquireLock acquires the lock.
//  1. Returns ErrLockCanceled if ctx is already cancelled
//  2. Locks the per-key mutex (serializes within the same process)
//  3. Reads DB refresh_started_at and checks for staleness (overwrite if stale)
//  4. Writes refresh_started_at=now, refresh_owner=ownerID to DB
func (m *LockManager) AcquireLock(ctx context.Context, profile, resource string) error {
	// check ctx cancellation (before acquiring mutex)
	if ctx.Err() != nil {
		return ErrLockCanceled
	}

	key := lockKey{Profile: profile, Resource: resource}
	keyMu := m.getKeyMutex(key)
	keyMu.Lock()

	// also check ctx after acquiring mutex
	if ctx.Err() != nil {
		keyMu.Unlock()
		return ErrLockCanceled
	}

	// DB stale check (for observing and recovering locks from other processes)
	// in-process mutex is the primary control; non-stale DB locks are also proceeded in MVP
	_, err := m.syncStore.Get(ctx, profile, resource)
	if err != nil {
		keyMu.Unlock()
		return err
	}

	// write lock info to DB
	if err := m.updater.MarkLockAcquired(ctx, profile, resource, m.ownerID, m.now()); err != nil {
		keyMu.Unlock()
		return err
	}

	return nil
}

// ReleaseLock releases the lock.
//  1. Resets refresh_started_at, refresh_owner to NULL in DB
//  2. Unlocks the per-key mutex
func (m *LockManager) ReleaseLock(ctx context.Context, profile, resource string) error {
	key := lockKey{Profile: profile, Resource: resource}

	// clear lock info in DB
	dbErr := m.updater.MarkLockReleased(ctx, profile, resource)

	// release per-key mutex (even if there is a DB error)
	m.mu.Lock()
	keyMu, ok := m.locks[key]
	m.mu.Unlock()
	if ok {
		keyMu.Unlock()
	}

	return dbErr
}

// TryAcquireLock attempts to acquire the lock without blocking.
// Returns *RefreshInProgressError if the lock is held by another holder
// (in-process mutex busy, or DB has a non-stale lock owned by someone else).
// If the DB lock is stale (older than staleLockTimeout), takeover succeeds.
//
// retryAfterSeconds は呼び出し元が応答に含める推奨値（固定値）。
func (m *LockManager) TryAcquireLock(ctx context.Context, profile, resource string, retryAfterSeconds int) error {
	if ctx.Err() != nil {
		return ErrLockCanceled
	}

	key := lockKey{Profile: profile, Resource: resource}
	keyMu := m.getKeyMutex(key)
	if !keyMu.TryLock() {
		// in-process holder. DB から holder/elapsed を読んで返す（best-effort）。
		holder, elapsed := m.dbHolderInfo(ctx, profile, resource)
		return &RefreshInProgressError{
			Profile:           profile,
			Resource:          resource,
			Holder:            holder,
			ElapsedSeconds:    elapsed,
			RetryAfterSeconds: retryAfterSeconds,
		}
	}

	if ctx.Err() != nil {
		keyMu.Unlock()
		return ErrLockCanceled
	}

	state, err := m.syncStore.Get(ctx, profile, resource)
	if err != nil {
		keyMu.Unlock()
		return err
	}

	// DB に他プロセスの active lock がある場合、stale でなければ 429 相当を返す。
	if state != nil && state.RefreshStartedAt.Valid && !m.isStale(state) {
		holder := ""
		if state.RefreshOwner.Valid {
			holder = state.RefreshOwner.String
		}
		// 自分自身（同 ownerID）が保持しているのは想定外（in-process mutex を抜けた直後）。
		// 念のため自分以外なら refresh in progress を返す。
		if holder != "" && holder != m.ownerID {
			elapsed, _ := elapsedSecondsSince(state.RefreshStartedAt.String, m.now())
			keyMu.Unlock()
			return &RefreshInProgressError{
				Profile:           profile,
				Resource:          resource,
				Holder:            holder,
				ElapsedSeconds:    elapsed,
				RetryAfterSeconds: retryAfterSeconds,
			}
		}
	}

	if err := m.updater.MarkLockAcquired(ctx, profile, resource, m.ownerID, m.now()); err != nil {
		keyMu.Unlock()
		return err
	}
	return nil
}

// TryWithLock is the non-blocking variant of WithLock. Returns *RefreshInProgressError
// when the lock is busy.
func (m *LockManager) TryWithLock(ctx context.Context, profile, resource string, retryAfterSeconds int, fn func() error) error {
	if err := m.TryAcquireLock(ctx, profile, resource, retryAfterSeconds); err != nil {
		return err
	}
	defer func() {
		_ = m.ReleaseLock(context.Background(), profile, resource)
	}()
	return fn()
}

// dbHolderInfo は sync_state から holder と経過秒を best-effort で取得する。
func (m *LockManager) dbHolderInfo(ctx context.Context, profile, resource string) (string, int) {
	state, err := m.syncStore.Get(ctx, profile, resource)
	if err != nil || state == nil {
		return "", 0
	}
	holder := ""
	if state.RefreshOwner.Valid {
		holder = state.RefreshOwner.String
	}
	elapsed := 0
	if state.RefreshStartedAt.Valid {
		elapsed, _ = elapsedSecondsSince(state.RefreshStartedAt.String, m.now())
	}
	return holder, elapsed
}

// elapsedSecondsSince は RFC3339 文字列から「now との差秒」を返す。
func elapsedSecondsSince(rfc3339Str string, now time.Time) (int, error) {
	t, err := time.Parse(time.RFC3339, rfc3339Str)
	if err != nil {
		return 0, err
	}
	return int(now.Sub(t).Seconds()), nil
}

// WithLock is a helper that guarantees: acquire lock → execute fn → release lock.
// Calls ReleaseLock via defer even if fn panics.
// Uses context.Background() to ensure ReleaseLock is called even after ctx cancellation.
func (m *LockManager) WithLock(ctx context.Context, profile, resource string, fn func() error) error {
	if err := m.AcquireLock(ctx, profile, resource); err != nil {
		return err
	}
	defer func() {
		_ = m.ReleaseLock(context.Background(), profile, resource)
	}()
	return fn()
}
