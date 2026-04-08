package refresh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/youyo/board/internal/cache"
)

// newLockTestDB はテスト用インメモリ DB を開くヘルパー。
func newLockTestDB(t *testing.T) (*cache.DB, *cache.SyncStateStore) {
	t.Helper()
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	return db, ss
}

// newLockManagerWithClock は差し込み可能な時計を持つ LockManager を生成するヘルパー。
func newLockManagerWithClock(ss *cache.SyncStateStore, ownerID string, nowFn func() time.Time) *LockManager {
	lm := NewLockManager(ss, ownerID)
	lm.now = nowFn
	return lm
}

// ---- Updater 拡張テスト ----

// TestUpdater_MarkLockAcquired_SetsFields: refresh_started_at と refresh_owner が設定される
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

// TestUpdater_MarkLockAcquired_NewRecord: sync_state が存在しなくても upsert が成功する
func TestUpdater_MarkLockAcquired_NewRecord(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// 事前状態なし
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

// TestUpdater_MarkLockReleased_ClearsFields: refresh_started_at, refresh_owner が NULL になる
func TestUpdater_MarkLockReleased_ClearsFields(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// まず取得する
	if err := u.MarkLockAcquired(ctx, "default", "clients", "host:1", now); err != nil {
		t.Fatalf("MarkLockAcquired: %v", err)
	}

	// 解放する
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

// TestUpdater_MarkLockReleased_NoRecord: sync_state が存在しなくてもエラーなし
func TestUpdater_MarkLockReleased_NoRecord(t *testing.T) {
	_, ss := newLockTestDB(t)
	u := NewUpdater(ss)
	ctx := context.Background()

	// 事前状態なしで解放
	err := u.MarkLockReleased(ctx, "nonexistent", "clients")
	if err != nil {
		t.Fatalf("MarkLockReleased on non-existent record: %v", err)
	}
}

// ---- LockManager テスト ----

// TestLockManager_AcquireLock_Success: DB に sync_state なしで AcquireLock が成功する
func TestLockManager_AcquireLock_Success(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	if err := lm.AcquireLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer func() { _ = lm.ReleaseLock(context.Background(), "default", "clients") }()

	// DB に refresh_started_at が設定されているか
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil || !state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be set after AcquireLock")
	}
}

// TestLockManager_ReleaseLock_ClearsDB: AcquireLock 後に ReleaseLock すると DB がクリアされる
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

// TestLockManager_WithLock_FnCalled: fn が呼ばれ、戻り値が fn の戻り値と等しい
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

// TestLockManager_WithLock_ReleasedOnFnError: fn がエラーを返しても DB ロックが解放される
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

	// DB ロックが解放されているか
	state, _ := ss.Get(ctx, "default", "clients")
	if state != nil && state.RefreshStartedAt.Valid {
		t.Error("RefreshStartedAt should be NULL after fn error")
	}
}

// TestLockManager_WithLock_ReleasedOnFnSuccess: fn が成功後に DB ロックが解放される
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

// ---- ctx キャンセルテスト ----

// TestLockManager_AcquireLock_CanceledContext: キャンセル済み ctx では ErrLockCanceled を返す
func TestLockManager_AcquireLock_CanceledContext(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即キャンセル

	err := lm.AcquireLock(ctx, "default", "clients")
	if !errors.Is(err, ErrLockCanceled) {
		t.Errorf("AcquireLock with canceled ctx = %v, want ErrLockCanceled", err)
	}
}

// ---- stale ロックテスト ----

// TestLockManager_isStale_BelowThreshold: 9分前は stale でない
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

// TestLockManager_isStale_AboveThreshold: 11分前は stale
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

// TestLockManager_isStale_NoStartedAt: RefreshStartedAt が NULL は stale でない
func TestLockManager_isStale_NoStartedAt(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })

	state := &cache.SyncState{} // RefreshStartedAt は zero value (Valid=false)
	if lm.isStale(state) {
		t.Error("isStale should be false when RefreshStartedAt is NULL")
	}
}

// TestLockManager_AcquireLock_StaleDetection_Overrides: 11分前の refresh_started_at を上書きできる
func TestLockManager_AcquireLock_StaleDetection_Overrides(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:new", func() time.Time { return now })
	ctx := context.Background()

	// 11分前の stale ロックを DB に設定
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

	// AcquireLock が成功するはず
	if err := lm.AcquireLock(ctx, "default", "clients"); err != nil {
		t.Fatalf("AcquireLock with stale lock: %v", err)
	}
	defer func() { _ = lm.ReleaseLock(context.Background(), "default", "clients") }()

	// refresh_started_at が新しい時刻に更新されているか
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

// ---- 並行処理テスト ----

// TestLockManager_WithLock_Sequential_SameKey: 同一 key の 2 goroutine が逐次実行される
func TestLockManager_WithLock_Sequential_SameKey(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	counter := 0
	var mu sync.Mutex // counter 保護用
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

		time.Sleep(20 * time.Millisecond) // 重複検出のための待機

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

// TestLockManager_WithLock_Parallel_DifferentKeys: 別 key の goroutine は並行実行できる
func TestLockManager_WithLock_Parallel_DifferentKeys(t *testing.T) {
	_, ss := newLockTestDB(t)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lm := newLockManagerWithClock(ss, "testhost:1", func() time.Time { return now })
	ctx := context.Background()

	// 別 key なら並行実行されるため、同一 key（20ms sleep × 2）より大幅に速いはず
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

	// 並行実行されるなら elapsed ≈ 30ms（直列なら 60ms+）
	// 余裕を持って 55ms 未満であれば並行実行されたと判断
	if elapsed >= 55*time.Millisecond {
		t.Errorf("fn for different keys should run in parallel, elapsed=%v (want < 55ms)", elapsed)
	}
}
