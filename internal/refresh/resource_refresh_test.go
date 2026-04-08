package refresh

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/cache"
)

// stubFetcher はテスト用の Fetcher 実装。
type stubFetcher struct {
	resource       string
	listAllItems   []json.RawMessage
	listSinceItems []json.RawMessage
	listAllErr     error
	listSinceErr   error
	capturedSince  string // ListUpdatedSince の引数記録
}

func (f *stubFetcher) ResourceName() string { return f.resource }

func (f *stubFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	return f.listAllItems, f.listAllErr
}

func (f *stubFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	f.capturedSince = since
	return f.listSinceItems, f.listSinceErr
}

// makeRaw は JSON 文字列から json.RawMessage を生成するヘルパー。
func makeRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}

// TestDeltaRefresh_NoExistingState_FetchesAll: sync_state なし → cursor="" でフェッチ、全件 UpsertMany
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
	// cursor="" で ListUpdatedSince が呼ばれた
	if fetcher.capturedSince != "" {
		t.Errorf("capturedSince = %q, want empty string", fetcher.capturedSince)
	}

	// キャッシュに2件保存されている
	entries, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("cache entries = %d, want 2", len(entries))
	}
}

// TestDeltaRefresh_WithCursor_PassesCursorToFetcher: cursor_updated_at あり → fetcher に since を渡す
func TestDeltaRefresh_WithCursor_PassesCursorToFetcher(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	// 既存の SyncState に cursor を設定
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

	// cursor が fetcher に渡された
	if fetcher.capturedSince != "2025-01-10T00:00:00Z" {
		t.Errorf("capturedSince = %q, want %q", fetcher.capturedSince, "2025-01-10T00:00:00Z")
	}
}

// TestDeltaRefresh_UpdatesMaxUpdatedAtAsCursor: 取得 entity の最大 updated_at が新カーソルになる
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

	// 最大 updated_at が新カーソル
	if result.NewCursor != "2025-01-15T09:00:00Z" {
		t.Errorf("NewCursor = %q, want %q", result.NewCursor, "2025-01-15T09:00:00Z")
	}

	// sync_state の cursor も更新されている
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.CursorUpdatedAt.String != "2025-01-15T09:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-15T09:00:00Z")
	}
}

// TestDeltaRefresh_EmptyResult_PreservesCursor: 差分 0 件 → カーソル変化なし
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
		listSinceItems: []json.RawMessage{}, // 0件
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

	// sync_state のカーソルは保持されている
	state, err := ss.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.CursorUpdatedAt.String != "2025-01-10T00:00:00Z" {
		t.Errorf("CursorUpdatedAt = %q, want %q", state.CursorUpdatedAt.String, "2025-01-10T00:00:00Z")
	}
}

// TestDeltaRefresh_UpdatesSyncState: last_synced_at, last_sync_mode, last_daily_refresh_date が正しく設定される
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

// TestDeltaRefresh_FetcherError_MarksError: fetcher エラー → MarkError が呼ばれ、エラー返却
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
	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}
}

// TestDeltaRefresh_EntityWithNoUpdatedAt_Handled: updated_at なし entity → カーソル計算でスキップ（エラーなし）
func TestDeltaRefresh_EntityWithNoUpdatedAt_Handled(t *testing.T) {
	db := openRefreshTestDB(t)
	rc := cache.NewResourceCache(db)
	ss := cache.NewSyncStateStore(db)
	r := NewRefresher(rc, ss)
	ctx := context.Background()

	fetcher := &stubFetcher{
		resource: "clients",
		listSinceItems: []json.RawMessage{
			makeRaw(`{"id":1}`), // updated_at なし
		},
	}

	result, err := r.DeltaRefresh(ctx, "default", fetcher, testNow, testTZ)
	if err != nil {
		t.Fatalf("DeltaRefresh: %v", err)
	}

	if result.FetchedCount != 1 {
		t.Errorf("FetchedCount = %d, want 1", result.FetchedCount)
	}
	// cursor は空のまま（updated_at なしはスキップ）
	if result.NewCursor != "" {
		t.Errorf("NewCursor = %q, want empty", result.NewCursor)
	}
}

// errFetchFailed はテスト用エラー
var errFetchFailed = errStr("fetch failed")

type errStr string

func (e errStr) Error() string { return string(e) }
