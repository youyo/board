package refresh

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/cache"
)

// TestForceRefresh_DeletesAllThenUpserts: DeleteAll 後に UpsertMany が呼ばれる順序保証
func TestForceRefresh_DeletesAllThenUpserts(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// 既存データを入れておく
	existing := cache.Entry{
		Key:         cache.EntityKey{Profile: "default", Resource: "clients", EntityID: "999"},
		PayloadJSON: json.RawMessage(`{"id":999}`),
	}
	if err := rc.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource: "clients",
		listAllItems: []json.RawMessage{
			makeRaw(`{"id":1,"updated_at":"2025-01-10T00:00:00Z"}`),
			makeRaw(`{"id":2,"updated_at":"2025-01-12T00:00:00Z"}`),
		},
	}

	result, err := r.ForceRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.FetchedCount != 2 {
		t.Errorf("FetchedCount = %d, want 2", result.FetchedCount)
	}

	// 既存データ（999）は削除されている
	entries, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("cache entries = %d, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Key.EntityID == "999" {
			t.Error("old entity 999 should be deleted by ForceRefresh")
		}
	}
}

// TestForceRefresh_UpdatesSyncState: last_full_synced_at, last_sync_mode="full", must_full_resync=false
func TestForceRefresh_UpdatesSyncState(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// MustFullResync=true でセット
	existing := cache.SyncState{
		ProfileName:    "default",
		ResourceName:   "clients",
		MustFullResync: true,
	}
	if err := ss.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource: "clients",
		listAllItems: []json.RawMessage{
			makeRaw(`{"id":1,"updated_at":"2025-01-15T00:00:00Z"}`),
		},
	}

	_, err := r.ForceRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !state.LastFullSyncedAt.Valid {
		t.Error("LastFullSyncedAt should be valid")
	}
	if state.LastSyncMode.String != "full" {
		t.Errorf("LastSyncMode = %q, want \"full\"", state.LastSyncMode.String)
	}
	if state.LastSyncStatus.String != "success" {
		t.Errorf("LastSyncStatus = %q, want \"success\"", state.LastSyncStatus.String)
	}
	if state.MustFullResync {
		t.Error("MustFullResync should be false after ForceRefresh")
	}
}

// TestForceRefresh_EmptyResult_ClearsCache: 全件0件 → DeleteAll のみ実行、cache は空
func TestForceRefresh_EmptyResult_ClearsCache(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// 既存データを入れておく
	existing := cache.Entry{
		Key:         cache.EntityKey{Profile: "default", Resource: "clients", EntityID: "1"},
		PayloadJSON: json.RawMessage(`{"id":1}`),
	}
	if err := rc.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource:     "clients",
		listAllItems: []json.RawMessage{}, // 0件
	}

	result, err := r.ForceRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}

	if result.FetchedCount != 0 {
		t.Errorf("FetchedCount = %d, want 0", result.FetchedCount)
	}

	// cache は空
	entries, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("cache entries = %d, want 0", len(entries))
	}
}

// TestForceRefresh_FetcherError_MarksError: fetcher エラー → MarkError が呼ばれ、DeleteAll は実行しない
func TestForceRefresh_FetcherError_MarksError(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// 既存データを入れておく（DeleteAll が呼ばれていないことを確認するため）
	existing := cache.Entry{
		Key:         cache.EntityKey{Profile: "default", Resource: "clients", EntityID: "1"},
		PayloadJSON: json.RawMessage(`{"id":1}`),
	}
	if err := rc.Upsert(ctx, existing); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetcher := &stubFetcher{
		resource:   "clients",
		listAllErr: errFetchFailed,
	}

	_, err := r.ForceRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// sync_state にエラーが記録されている
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

	// 既存データは保持されている（DeleteAll は呼ばれていない）
	entries, err2 := rc.List(ctx, "default", "clients")
	if err2 != nil {
		t.Fatalf("List: %v", err2)
	}
	if len(entries) != 1 {
		t.Errorf("cache entries = %d, want 1 (DeleteAll should not be called on error)", len(entries))
	}
}

// TestForceRefresh_ResetsCursor: 完了後 cursor_updated_at が NULL
func TestForceRefresh_ResetsCursor(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// 既存カーソルをセット
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
		listAllItems: []json.RawMessage{
			makeRaw(`{"id":1,"updated_at":"2025-01-15T00:00:00Z"}`),
		},
	}

	_, err := r.ForceRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("ForceRefresh: %v", err)
	}

	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// cursor は NULL にリセット
	if state.CursorUpdatedAt.Valid {
		t.Errorf("CursorUpdatedAt should be NULL, got %q", state.CursorUpdatedAt.String)
	}
}
