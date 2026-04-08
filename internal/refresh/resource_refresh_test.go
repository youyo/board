package refresh

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/cache"
)

// stubFetcher is a test implementation of Fetcher.
type stubFetcher struct {
	resource       string
	listAllItems   []json.RawMessage
	listSinceItems []json.RawMessage
	listAllErr     error
	listSinceErr   error
	capturedSince  string // records the argument passed to ListUpdatedSince
}

func (f *stubFetcher) ResourceName() string { return f.resource }

func (f *stubFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	return f.listAllItems, f.listAllErr
}

func (f *stubFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	f.capturedSince = since
	return f.listSinceItems, f.listSinceErr
}

// makeRaw is a helper that creates a json.RawMessage from a JSON string.
func makeRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}

// TestDeltaRefresh_NoExistingState_FetchesAll: no sync_state → fetch with cursor="", UpsertMany all items
func TestDeltaRefresh_NoExistingState_FetchesAll(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource: "clients",
		listSinceItems: []json.RawMessage{
			makeRaw(`{"id":1,"updated_at":"2025-01-10T00:00:00Z"}`),
			makeRaw(`{"id":2,"updated_at":"2025-01-12T00:00:00Z"}`),
		},
	}

	result, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.FetchedCount != 2 {
		t.Errorf("FetchedCount = %d, want 2", result.FetchedCount)
	}
	// ListUpdatedSince was called with cursor=""
	if fetcher.capturedSince != "" {
		t.Errorf("capturedSince = %q, want empty string", fetcher.capturedSince)
	}

	// 2 items are saved in cache
	entries, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("cache entries = %d, want 2", len(entries))
	}
}

// TestDeltaRefresh_WithCursor_PassesCursorToFetcher: cursor_updated_at exists → pass since to fetcher
func TestDeltaRefresh_WithCursor_PassesCursorToFetcher(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// set cursor in existing SyncState
	existing := cache.SyncState{
		ProfileName:     "default",
		ResourceName:    "clients",
		CursorUpdatedAt: nullString("2025-01-10T00:00:00Z"),
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource: "clients",
		listSinceItems: []json.RawMessage{
			makeRaw(`{"id":3,"updated_at":"2025-01-15T00:00:00Z"}`),
		},
	}

	_, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	// cursor was passed to fetcher
	if fetcher.capturedSince != "2025-01-10T00:00:00Z" {
		t.Errorf("capturedSince = %q, want %q", fetcher.capturedSince, "2025-01-10T00:00:00Z")
	}
}

// TestDeltaRefresh_UpdatesMaxUpdatedAtAsCursor: maximum updated_at of fetched entities becomes new cursor
func TestDeltaRefresh_UpdatesMaxUpdatedAtAsCursor(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource: "clients",
		listSinceItems: []json.RawMessage{
			makeRaw(`{"id":1,"updated_at":"2025-01-10T00:00:00Z"}`),
			makeRaw(`{"id":2,"updated_at":"2025-01-15T09:00:00Z"}`),
			makeRaw(`{"id":3,"updated_at":"2025-01-12T00:00:00Z"}`),
		},
	}

	result, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	// maximum updated_at is the new cursor
	if result.NewCursor != "2025-01-15T09:00:00Z" {
		t.Errorf("NewCursor = %q, want %q", result.NewCursor, "2025-01-15T09:00:00Z")
	}

	// sync_state cursor is also updated
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.CursorUpdatedAt.String != "2025-01-15T09:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-15T09:00:00Z")
	}
}

// TestDeltaRefresh_EmptyResult_PreservesCursor: 0 delta items → cursor unchanged
func TestDeltaRefresh_EmptyResult_PreservesCursor(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	existing := cache.SyncState{
		ProfileName:     "default",
		ResourceName:    "clients",
		CursorUpdatedAt: nullString("2025-01-10T00:00:00Z"),
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource:       "clients",
		listSinceItems: []json.RawMessage{}, // 0 items
	}

	result, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	if result.FetchedCount != 0 {
		t.Errorf("FetchedCount = %d, want 0", result.FetchedCount)
	}
	if result.NewCursor != "" {
		t.Errorf("NewCursor = %q, want empty (no change)", result.NewCursor)
	}

	// sync_state cursor is preserved
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.CursorUpdatedAt.String != "2025-01-10T00:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-10T00:00:00Z")
	}
}

// TestDeltaRefresh_UpdatesSyncState: last_synced_at, last_sync_mode, last_daily_refresh_date are correctly set
func TestDeltaRefresh_UpdatesSyncState(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource:       "clients",
		listSinceItems: []json.RawMessage{},
	}

	_, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !state.LastSyncedAt.Valid {
		t.Error("LastSyncedAt should be valid")
	}
	if state.LastSyncMode.String != "delta" {
		t.Errorf("LastSyncMode = %q, want \"delta\"", state.LastSyncMode.String)
	}
	wantDate := TodayInTZ(testNow, testTZ)
	if state.LastDailyRefreshDate.String != wantDate {
		t.Errorf("LastDailyRefreshDate = %q, want %q", state.LastDailyRefreshDate.String, wantDate)
	}
}

// TestDeltaRefresh_FetcherError_MarksError: fetcher error → MarkError is called and error is returned
func TestDeltaRefresh_FetcherError_MarksError(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource:     "clients",
		listSinceErr: errFetchFailed,
	}

	_, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// error is recorded in sync_state
	state, err2 := ss.Get(ctx, "default", "clients")
	if err2 != nil {
		t.Fatalf("Get: %v", err2)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.LastSyncStatus.String != "error" {
		t.Errorf("LastSyncStatus = %q, want \"error\"", state.LastSyncStatus.String)
	}
	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}
}

// TestDeltaRefresh_EntityWithNoUpdatedAt_Handled: entity without updated_at → skipped in cursor calculation (no error)
func TestDeltaRefresh_EntityWithNoUpdatedAt_Handled(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource: "clients",
		listSinceItems: []json.RawMessage{
			makeRaw(`{"id":1}`), // no updated_at
		},
	}

	result, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	if result.FetchedCount != 1 {
		t.Errorf("FetchedCount = %d, want 1", result.FetchedCount)
	}
	// cursor remains empty (entities without updated_at are skipped)
	if result.NewCursor != "" {
		t.Errorf("NewCursor = %q, want empty", result.NewCursor)
	}
}

// errFetchFailed is a test error
var errFetchFailed = errStr("fetch failed")

type errStr string

func (e errStr) Error() string { return string(e) }
