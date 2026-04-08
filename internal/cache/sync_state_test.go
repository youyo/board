package cache

import (
	"context"
	"database/sql"
	"testing"
)

// makeSyncState はテスト用 SyncState を生成する。
func makeSyncState(profile, resource string) SyncState {
	return SyncState{
		ProfileName:          profile,
		ResourceName:         resource,
		LastSyncedAt:         sql.NullString{String: "2024-01-01T00:00:00Z", Valid: true},
		CursorUpdatedAt:      sql.NullString{String: "2024-01-01T00:00:00Z", Valid: true},
		LastFullSyncedAt:     sql.NullString{Valid: false},
		LastSyncMode:         sql.NullString{String: "delta", Valid: true},
		LastSyncStatus:       sql.NullString{String: "ok", Valid: true},
		LastDailyRefreshDate: sql.NullString{String: "2024-01-01", Valid: true},
		MustFullResync:       false,
		ExpiredAt:            sql.NullString{Valid: false},
		InvalidateReason:     sql.NullString{Valid: false},
		LastErrorAt:          sql.NullString{Valid: false},
		LastErrorCode:        sql.NullString{Valid: false},
		LastErrorMessage:     sql.NullString{Valid: false},
		ConsecutiveFailures:  0,
		RefreshStartedAt:     sql.NullString{Valid: false},
		RefreshOwner:         sql.NullString{Valid: false},
		CacheVersion:         1,
		SchemaVersion:        1,
	}
}

// T_SS01: NewSyncStateStore が non-nil を返す
func TestNewSyncStateStore(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	if s == nil {
		t.Fatal("NewSyncStateStore returned nil")
	}
}

// T_SS02: Get が存在しないキーに対して nil, nil を返す
func TestSyncState_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	got, err := s.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("Get returned %+v, want nil", got)
	}
}

// T_SS03: Upsert→Get が正しく値を返す
func TestSyncState_UpsertAndGet(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	state := makeSyncState("default", "clients")
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.ProfileName != "default" {
		t.Errorf("ProfileName: got %q, want %q", got.ProfileName, "default")
	}
	if got.ResourceName != "clients" {
		t.Errorf("ResourceName: got %q, want %q", got.ResourceName, "clients")
	}
	if got.LastSyncMode.String != "delta" {
		t.Errorf("LastSyncMode: got %q, want %q", got.LastSyncMode.String, "delta")
	}
	if got.CacheVersion != 1 {
		t.Errorf("CacheVersion: got %d, want 1", got.CacheVersion)
	}
}

// T_SS04: Upsert が既存エントリを上書きする
func TestSyncState_UpsertOverwrite(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	state := makeSyncState("default", "clients")
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert initial: %v", err)
	}

	state.LastSyncMode = sql.NullString{String: "full", Valid: true}
	state.CacheVersion = 2
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert overwrite: %v", err)
	}

	got, err := s.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastSyncMode.String != "full" {
		t.Errorf("LastSyncMode: got %q, want %q", got.LastSyncMode.String, "full")
	}
	if got.CacheVersion != 2 {
		t.Errorf("CacheVersion: got %d, want 2", got.CacheVersion)
	}
}

// T_SS05: Delete が指定エントリを削除する
func TestSyncState_Delete(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	state := makeSyncState("default", "clients")
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := s.Delete(ctx, "default", "clients"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if got != nil {
		t.Error("entry should be deleted")
	}
}

// T_SS06: MustFullResync の bool→INTEGER→bool 変換が正しく動作する
func TestSyncState_MustFullResync(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	state := makeSyncState("default", "projects")
	state.MustFullResync = true
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "default", "projects")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if !got.MustFullResync {
		t.Error("MustFullResync: got false, want true")
	}

	// false に更新
	state.MustFullResync = false
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert false: %v", err)
	}

	got2, err := s.Get(ctx, "default", "projects")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got2.MustFullResync {
		t.Error("MustFullResync: got true, want false")
	}
}

// T_SS07: NULL 許容フィールドが正しくスキャンされる
func TestSyncState_NullFields(t *testing.T) {
	db := openTestDB(t)
	s := NewSyncStateStore(db)
	ctx := context.Background()

	state := SyncState{
		ProfileName:         "default",
		ResourceName:        "vendors",
		ConsecutiveFailures: 0,
		CacheVersion:        1,
		SchemaVersion:       1,
		// 残りは全て NullString{Valid: false}
	}
	if err := s.Upsert(ctx, state); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "default", "vendors")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.LastSyncedAt.Valid {
		t.Error("LastSyncedAt should be NULL")
	}
	if got.LastErrorMessage.Valid {
		t.Error("LastErrorMessage should be NULL")
	}
	if got.RefreshOwner.Valid {
		t.Error("RefreshOwner should be NULL")
	}
}
