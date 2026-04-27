package cache_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/youyo/board/internal/cache"
)

func setupDB(t *testing.T) (*cache.DB, *cache.SyncStateStore) {
	t.Helper()
	db, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := cache.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, cache.NewSyncStateStore(db)
}

func TestLoadInfos_NoState(t *testing.T) {
	_, ss := setupDB(t)
	got := cache.LoadInfos(context.Background(), ss, "default", []string{"clients", "projects"})
	if len(got) != 2 {
		t.Fatalf("want 2 infos, got %d", len(got))
	}
	for _, info := range got {
		if info.CachedAt != "" || info.FullRefreshedAt != "" {
			t.Errorf("expected empty timestamps for missing state: %+v", info)
		}
	}
}

func TestLoadInfos_WithState(t *testing.T) {
	_, ss := setupDB(t)
	if err := ss.Upsert(context.Background(), cache.SyncState{
		ProfileName:      "default",
		ResourceName:     "clients",
		LastSyncedAt:     sql.NullString{String: "2026-04-25T10:00:00Z", Valid: true},
		LastFullSyncedAt: sql.NullString{String: "2026-04-01T12:00:00Z", Valid: true},
		CacheVersion:     1,
		SchemaVersion:    1,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got := cache.LoadInfos(context.Background(), ss, "default", []string{"clients", "projects"})
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Resource != "clients" || got[0].CachedAt != "2026-04-25T10:00:00Z" || got[0].FullRefreshedAt != "2026-04-01T12:00:00Z" {
		t.Errorf("clients info wrong: %+v", got[0])
	}
	if got[1].Resource != "projects" || got[1].CachedAt != "" {
		t.Errorf("projects info wrong: %+v", got[1])
	}
}

func TestLoadInfos_EmptyResources(t *testing.T) {
	_, ss := setupDB(t)
	got := cache.LoadInfos(context.Background(), ss, "default", nil)
	if got == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("want empty slice, got %d items", len(got))
	}
}

func TestLoadInfos_NilStore(t *testing.T) {
	got := cache.LoadInfos(context.Background(), nil, "default", []string{"clients"})
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].CachedAt != "" {
		t.Errorf("expected empty CachedAt with nil store, got %q", got[0].CachedAt)
	}
}
