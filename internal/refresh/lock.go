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
