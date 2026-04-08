package refresh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/youyo/board/internal/cache"
)

// newLockTestDB is a helper that opens an in-memory DB for testing.
func newLockTestDB(t *testing.T) (*cache.DB, *cache.SyncStateStore) {
	t.Helper()
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	return db, ss
}

// newLockManagerWithClock is a helper that creates a LockManager with an injectable clock.
func newLockManagerWithClock(ss *cache.SyncStateStore, ownerID string, nowFn func() time.Time) *LockManager {
	lm := NewLockManager(ss, ownerID)
	lm.now = nowFn
	return lm
}

// ---- Updater extension tests ----

// TestUpdater_MarkLockAcquired_SetsFields: refresh_started_at and refresh_owner are set
func TestUpdater_MarkLockAcquired_SetsFields(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	ownerID := "testhost:1234"

	if err := u.MarkLockAcquired(ctx, "default", "clients", ownerID, now); err != nil {
		t.Fatalf("MarkLockAcquired: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}

	wantTime := now.UTC().Format(time.RFC3339)
	if !state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be valid")
	}
	if state.RefreshStartedAt.String != wantTime {
		t.Errorf("RefreshStartedAt = %q, want %q", state.RefreshStartedAt.String, wantTime)
	}
	if !state.RefreshOwner.Valid {
		t.Error("RefreshOwner should be valid")
	}
	if state.RefreshOwner.String != ownerID {
		t.Errorf("RefreshOwner = %q, want %q", state.RefreshOwner.String, ownerID)
	}
}

// TestUpdater_MarkLockAcquired_NewRecord: upsert succeeds even when sync_state does not exist
func TestUpdater_MarkLockAcquired_NewRecord(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// no pre-existing state
	err := u.MarkLockAcquired(ctx, "newprofile", "projects", "host:999", now)
	if err != nil {
		t.Fatalf("MarkLockAcquired on new record: %v", err)
	}

	state, err := ss.Get(ctx, "newprofile", "projects")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil {
		t.Fatal("expected state to be created, got nil")
	}
	if !state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be valid")
	}
}

// TestUpdater_MarkLockReleased_ClearsFields: refresh_started_at, refresh_owner become NULL
func TestUpdater_MarkLockReleased_ClearsFields(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// acquire first
	if err := u.MarkLockAcquired(ctx, "default", "clients", "host:1", now); err != nil {
		t.Fatalf("MarkLockAcquired: %v", err)
	}

	// release
	if err := u.MarkLockReleased(ctx, "default", "clients"); err != nil {
		t.Fatalf("MarkLockReleased: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.RefreshStartedAt.Valid {
		t.Errorf("RefreshStartedAt should be NULL, got %q", state.RefreshStartedAt.String)
	}
	if state.RefreshOwner.Valid {
		t.Errorf("RefreshOwner should be NULL, got %q", state.RefreshOwner.String)
	}
}

// TestUpdater_MarkLockReleased_NoRecord: no error even when sync_state does not exist
func TestUpdater_MarkLockReleased_NoRecord(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	// release without pre-existing state
	err := u.MarkLockReleased(ctx, "nonexistent", "clients")
	if err != nil {
		t.Fatalf("MarkLockReleased on non-existent record: %v", err)
	}
}

// ---- LockManager tests ----

// TestLockManager_AcquireLock_Success: AcquireLock succeeds with no sync_state in DB
func TestLockManager_AcquireLock_Success(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	if err := lm.AcquireLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer func() { _ = lm.ReleaseLock(context.Background(), "default", "clients") }()

	// is refresh_started_at set in DB?
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil || !state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be set after AcquireLock")
	}
}

// TestLockManager_ReleaseLock_ClearsDB: ReleaseLock after AcquireLock clears the DB
func TestLockManager_ReleaseLock_ClearsDB(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	if err := lm.AcquireLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if err := lm.ReleaseLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state != nil && state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be NULL after ReleaseLock")
	}
	if state != nil && state.RefreshOwner.Valid {
		t.Error("RefreshOwner should be NULL after ReleaseLock")
	}
}

// TestLockManager_WithLock_FnCalled: fn is called and return value equals fn's return value
func TestLockManager_WithLock_FnCalled(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	called := false
	err := lm.WithLock(ctx, "default", "clients", func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}
	if !called {
		t.Error("fn should have been called")
	}
}

// TestLockManager_WithLock_ReleasedOnFnError: DB lock is released even when fn returns error
func TestLockManager_WithLock_ReleasedOnFnError(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	wantErr := errors.New("fn error")
	err := lm.WithLock(ctx, "default", "clients", func() error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("WithLock error = %v, want %v", err, wantErr)
	}

	// is DB lock released?
	state, _ := ss.Get(ctx, "default", "clients")
	if state != nil && state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be NULL after fn error")
	}
}

// TestLockManager_WithLock_ReleasedOnFnSuccess: DB lock is released after fn succeeds
func TestLockManager_WithLock_ReleasedOnFnSuccess(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	err := lm.WithLock(ctx, "default", "clients", func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}

	state, _ := ss.Get(ctx, "default", "clients")
	if state != nil && state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be NULL after WithLock success")
	}
}

// ---- ctx cancellation tests ----

// TestLockManager_AcquireLock_CanceledContext: ErrLockCanceled is returned for cancelled ctx
func TestLockManager_AcquireLock_CanceledContext(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := lm.AcquireLock(ctx, "default", "clients")
	if !errors.Is(err, ErrLockCanceled) {
		t.Errorf("AcquireLock with canceled ctx = %v, want ErrLockCanceled", err)
	}
}

// ---- stale lock tests ----

// TestLockManager_isStale_BelowThreshold: 9 minutes ago is not stale
func TestLockManager_isStale_BelowThreshold(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	state := &cache.SyncState{
		RefreshStartedAt: nullString(now.Add(-9 * time.Minute).UTC().Format(time.RFC3339)),
	}
	if lm.isStale(state) {
		t.Error("isStale should be false for 9 minutes ago")
	}
}

// TestLockManager_isStale_AboveThreshold: 11 minutes ago is stale
func TestLockManager_isStale_AboveThreshold(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	state := &cache.SyncState{
		RefreshStartedAt: nullString(now.Add(-11 * time.Minute).UTC().Format(time.RFC3339)),
	}
	if !lm.isStale(state) {
		t.Error("isStale should be true for 11 minutes ago")
	}
}

// TestLockManager_isStale_NoStartedAt: NULL RefreshStartedAt is not stale
func TestLockManager_isStale_NoStartedAt(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	state := &cache.SyncState{} // RefreshStartedAt is zero value (Valid=false)
	if lm.isStale(state) {
		t.Error("isStale should be false when RefreshStartedAt is NULL")
	}
}

// TestLockManager_AcquireLock_StaleDetection_Overrides: can overwrite refresh_started_at from 11 minutes ago
func TestLockManager_AcquireLock_StaleDetection_Overrides(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:new", func() time.Time { return now })
	ctx := context.Background()

	// set stale lock from 11 minutes ago in DB
	staleTime := now.Add(-11 * time.Minute)
	existing := cache.SyncState{
		ProfileName:      "default",
		ResourceName:     "clients",
		RefreshStartedAt: nullString(staleTime.UTC().Format(time.RFC3339)),
		RefreshOwner:     nullString("oldhost:999"),
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// AcquireLock should succeed
	if err := lm.AcquireLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("AcquireLock with stale lock: %v", err)
	}
	defer func() { _ = lm.ReleaseLock(context.Background(), "default", "clients") }()

	// is refresh_started_at updated to the new time?
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	wantTime := now.UTC().Format(time.RFC3339)
	if state.RefreshStartedAt.String != wantTime {
		t.Errorf("RefreshStartedAt = %q, want %q (new time)", state.RefreshStartedAt.String, wantTime)
	}
	if state.RefreshOwner.String != "testhost:new" {
		t.Errorf("RefreshOwner = %q, want %q", state.RefreshOwner.String, "testhost:new")
	}
}

// ---- concurrency tests ----

// TestLockManager_WithLock_Sequential_SameKey: 2 goroutines with the same key execute sequentially
func TestLockManager_WithLock_Sequential_SameKey(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	counter := 0
	var mu sync.Mutex // protects counter
	concurrent := false
	detected := false

	run := func() error {
		mu.Lock()
		counter++
		if counter > 1 {
			concurrent = true
			detected = true
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond) // wait to detect overlap

		mu.Lock()
		counter--
		mu.Unlock()
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = lm.WithLock(ctx, "default", "clients", run)
	}()
	go func() {
		defer wg.Done()
		_ = lm.WithLock(ctx, "default", "clients", run)
	}()
	wg.Wait()

	if detected && concurrent {
		t.Error("fn should not be executed concurrently for the same key")
	}
}

// TestLockManager_WithLock_Parallel_DifferentKeys: goroutines with different keys can execute in parallel
func TestLockManager_WithLock_Parallel_DifferentKeys(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	// different keys allow parallel execution, so should be significantly faster than same key (20ms sleep × 2)
	sleepDuration := 30 * time.Millisecond

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = lm.WithLock(ctx, "default", "clients", func() error {
			time.Sleep(sleepDuration)
			return nil
		})
	}()
	go func() {
		defer wg.Done()
		_ = lm.WithLock(ctx, "default", "projects", func() error {
			time.Sleep(sleepDuration)
			return nil
		})
	}()
	wg.Wait()
	elapsed := time.Since(start)

	// parallel execution: elapsed ≈ 30ms (sequential would be 60ms+)
	// if elapsed < 55ms with margin, parallel execution is confirmed
	if elapsed >= 55*time.Millisecond {
		t.Errorf("fn for different keys should run in parallel, elapsed=%v (want < 55ms)", elapsed)
	}
}
