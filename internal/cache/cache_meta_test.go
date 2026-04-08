package cache

import (
	"context"
	"testing"
)

// T_CM01: NewCacheMetaStore が non-nil を返す
func TestNewCacheMetaStore(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	if s == nil {
		t.Fatal("NewCacheMetaStore returned nil")
	}
}

// T_CM02: Set→Get が正しく値を返す
func TestCacheMeta_SetAndGet(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	ctx := context.Background()

	if err := s.Set(ctx, "foo", "bar"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := s.Get(ctx, "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "bar" {
		t.Errorf("Get: got %q, want %q", got, "bar")
	}
}

// T_CM03: Get が存在しないキーに対して "", nil を返す
func TestCacheMeta_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "" {
		t.Errorf("Get: got %q, want empty string", got)
	}
}

// T_CM04: Delete が指定キーを削除する
func TestCacheMeta_Delete(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	ctx := context.Background()

	if err := s.Set(ctx, "to_delete", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete(ctx, "to_delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "to_delete")
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if got != "" {
		t.Errorf("Get after Delete: got %q, want empty string", got)
	}
}

// T_CM05: Set が既存キーを上書きする
func TestCacheMeta_SetOverwrite(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	ctx := context.Background()

	if err := s.Set(ctx, "key", "old"); err != nil {
		t.Fatalf("Set initial: %v", err)
	}
	if err := s.Set(ctx, "key", "new"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}

	got, err := s.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "new" {
		t.Errorf("Get: got %q, want %q", got, "new")
	}
}

// T_CM06: Migrate 後に db_schema_version が正しく設定される
func TestCacheMeta_DBSchemaVersionAfterMigrate(t *testing.T) {
	db := openTestDB(t)
	s := NewCacheMetaStore(db)
	ctx := context.Background()

	got, err := s.Get(ctx, "db_schema_version")
	if err != nil {
		t.Fatalf("Get db_schema_version: %v", err)
	}
	if got == "" {
		t.Error("db_schema_version should be set after Migrate")
	}
}
