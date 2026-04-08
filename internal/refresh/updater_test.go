package refresh

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/cache"
)

// openRefreshTestDB is a helper that opens an in-memory DB for testing and applies migration.
func openRefreshTestDB(t *testing.T) *cache.DB {
	t.Helper()
	db, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	if err := cache.Migrate(db); err != nil {
		t.Fatalf("cache.Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var (
	testNow = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	testTZ  = time.UTC
)

// TestUpdater_MarkDeltaSuccess_NewRecord: first time with no sync_state, all fields are correctly set
func TestUpdater_MarkDeltaSuccess_NewRecord(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	err := u.MarkDeltaSuccess(ctx, "default", "clients", "2025-01-15T09:00:00Z", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkDeltaSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}

	if !state.LastSyncedAt.Valid {
		t.Error("LastSyncedAt should be valid")
	}
	if state.LastSyncMode.String != "delta" {
		t.Errorf("LastSyncMode = %q, want \"delta\"", state.LastSyncMode.String)
	}
	if state.LastSyncStatus.String != "success" {
		t.Errorf("LastSyncStatus = %q, want \"success\"", state.LastSyncStatus.String)
	}
	if !state.LastDailyRefreshDate.Valid {
		t.Error("LastDailyRefreshDate should be valid")
	}
	wantDate := TodayInTZ(testNow, testTZ)
	if state.LastDailyRefreshDate.String != wantDate {
		t.Errorf("LastDailyRefreshDate = %q, want %q", state.LastDailyRefreshDate.String, wantDate)
	}
	if state.CursorUpdatedAt.String != "2025-01-15T09:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-15T09:00:00Z")
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", state.ConsecutiveFailures)
	}
}

// TestUpdater_MarkDeltaSuccess_UpdatesCursor: cursor_updated_at of existing sync_state is updated
func TestUpdater_MarkDeltaSuccess_UpdatesCursor(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	// set existing state
	existing := cache.SyncState{
		ProfileName:         "default",
		ResourceName:        "clients",
		CursorUpdatedAt:     nullString("2025-01-01T00:00:00Z"),
		ConsecutiveFailures: 3,
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err := u.MarkDeltaSuccess(ctx, "default", "clients", "2025-01-15T09:00:00Z", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkDeltaSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.CursorUpdatedAt.String != "2025-01-15T09:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-15T09:00:00Z")
	}
}

// TestUpdater_MarkDeltaSuccess_EmptyCursorPreservesExisting: existing cursor_updated_at is retained when newCursor is empty
func TestUpdater_MarkDeltaSuccess_EmptyCursorPreservesExisting(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:     "default",
		ResourceName:    "clients",
		CursorUpdatedAt: nullString("2025-01-10T00:00:00Z"),
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// newCursor is empty
	err := u.MarkDeltaSuccess(ctx, "default", "clients", "", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkDeltaSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// existing cursor is retained
	if state.CursorUpdatedAt.String != "2025-01-10T00:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-10T00:00:00Z")
	}
}

// TestUpdater_MarkDeltaSuccess_SetsConsecutiveFailuresToZero: consecutive_failures is reset
func TestUpdater_MarkDeltaSuccess_SetsConsecutiveFailuresToZero(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:         "default",
		ResourceName:        "clients",
		ConsecutiveFailures: 5,
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err := u.MarkDeltaSuccess(ctx, "default", "clients", "", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkDeltaSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", state.ConsecutiveFailures)
	}
}

// TestUpdater_MarkForceSuccess_ResetsCursor: cursor_updated_at is reset to NULL
func TestUpdater_MarkForceSuccess_ResetsCursor(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:     "default",
		ResourceName:    "clients",
		CursorUpdatedAt: nullString("2025-01-10T00:00:00Z"),
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err := u.MarkForceSuccess(ctx, "default", "clients", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkForceSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.CursorUpdatedAt.Valid {
		t.Errorf("CursorUpdatedAt should be NULL, got %q", state.CursorUpdatedAt.String)
	}
}

// TestUpdater_MarkForceSuccess_SetsLastFullSyncedAt: last_full_synced_at is updated
func TestUpdater_MarkForceSuccess_SetsLastFullSyncedAt(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	err := u.MarkForceSuccess(ctx, "default", "clients", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkForceSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !state.LastFullSyncedAt.Valid {
		t.Error("LastFullSyncedAt should be valid")
	}
	wantNow := testNow.UTC().Format(time.RFC3339)
	if state.LastFullSyncedAt.String != wantNow {
		t.Errorf("LastFullSyncedAt = %q, want %q", state.LastFullSyncedAt.String, wantNow)
	}
	if state.LastSyncMode.String != "full" {
		t.Errorf("LastSyncMode = %q, want \"full\"", state.LastSyncMode.String)
	}
	if state.LastSyncStatus.String != "success" {
		t.Errorf("LastSyncStatus = %q, want \"success\"", state.LastSyncStatus.String)
	}
}

// TestUpdater_MarkForceSuccess_ClearsMustFullResync: must_full_resync is cleared to false
func TestUpdater_MarkForceSuccess_ClearsMustFullResync(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:    "default",
		ResourceName:   "clients",
		MustFullResync: true,
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err := u.MarkForceSuccess(ctx, "default", "clients", testNow, testTZ)
	if err != nil {
		t.Fatalf("MarkForceSuccess: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.MustFullResync {
		t.Error("MustFullResync should be false after ForceSuccess")
	}
}

// TestUpdater_MarkError_IncrementsConsecutiveFailures: consecutive_failures is incremented
func TestUpdater_MarkError_IncrementsConsecutiveFailures(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:         "default",
		ResourceName:        "clients",
		ConsecutiveFailures: 2,
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	err := u.MarkError(ctx, "default", "clients", "500", "internal error", testNow)
	if err != nil {
		t.Fatalf("MarkError: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.ConsecutiveFailures != 3 {
		t.Errorf("ConsecutiveFailures = %d, want 3", state.ConsecutiveFailures)
	}
}

// TestUpdater_MarkError_SetsErrorFields: last_error_at/code/message are updated
func TestUpdater_MarkError_SetsErrorFields(t *testing.T) {
	db := openRefreshTestDB(t)
	ss := cache.NewSyncStateStore(db)
	u := NewUpdater(ss)
	ctx := context.Background()

	err := u.MarkError(ctx, "default", "clients", "429", "rate limit exceeded", testNow)
	if err != nil {
		t.Fatalf("MarkError: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if state.LastSyncStatus.String != "error" {
		t.Errorf("LastSyncStatus = %q, want \"error\"", state.LastSyncStatus.String)
	}
	if !state.LastErrorAt.Valid {
		t.Error("LastErrorAt should be valid")
	}
	if state.LastErrorCode.String != "429" {
		t.Errorf("LastErrorCode = %q, want \"429\"", state.LastErrorCode.String)
	}
	if state.LastErrorMessage.String != "rate limit exceeded" {
		t.Errorf("LastErrorMessage = %q, want \"rate limit exceeded\"", state.LastErrorMessage.String)
	}
	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}
}
