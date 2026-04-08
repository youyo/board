package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
)

// openTestDB はテスト用インメモリ DB を開き、マイグレーションを適用する。
func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// makeEntry はテスト用 Entry を生成する。
func makeEntry(profile, resource, entityID string, payload string) Entry {
	return Entry{
		Key:         NewEntityKey(profile, resource, entityID),
		PayloadJSON: json.RawMessage(payload),
		UpdatedAt:   sql.NullString{String: "2024-01-01T00:00:00Z", Valid: true},
	}
}

// T_RC01: NewResourceCache が non-nil を返す
func TestNewResourceCache(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	if rc == nil {
		t.Fatal("NewResourceCache returned nil")
	}
}

// T_RC02: Upsert が新規エントリを挿入する
func TestUpsert_Insert(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entry := makeEntry("default", "clients", "1", `{"id":1}`)
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := rc.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Key.EntityID != "1" {
		t.Errorf("EntityID: got %q, want %q", got.Key.EntityID, "1")
	}
}

// T_RC03: Upsert が既存エントリを上書き（REPLACE）する
func TestUpsert_Replace(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entry := makeEntry("default", "clients", "1", `{"id":1,"name":"old"}`)
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert initial: %v", err)
	}

	entry.PayloadJSON = json.RawMessage(`{"id":1,"name":"new"}`)
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert replace: %v", err)
	}

	got, err := rc.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.PayloadJSON) != `{"id":1,"name":"new"}` {
		t.Errorf("PayloadJSON: got %s, want %s", got.PayloadJSON, `{"id":1,"name":"new"}`)
	}
}

// T_RC04: Upsert が FetchedAt を自動設定する
func TestUpsert_FetchedAt(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entry := makeEntry("default", "clients", "1", `{"id":1}`)
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := rc.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FetchedAt == "" {
		t.Error("FetchedAt should be set automatically")
	}
}

// T_RC05: Get が存在しないキーに対して nil, nil を返す
func TestGet_NotFound(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	got, err := rc.Get(ctx, NewEntityKey("default", "clients", "nonexistent"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("Get returned %+v, want nil", got)
	}
}

// T_RC06: Get が updated_at NULL を正しくスキャンする
func TestGet_NullUpdatedAt(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entry := Entry{
		Key:         NewEntityKey("default", "clients", "2"),
		PayloadJSON: json.RawMessage(`{"id":2}`),
		UpdatedAt:   sql.NullString{Valid: false}, // NULL
	}
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := rc.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UpdatedAt.Valid {
		t.Error("UpdatedAt should be NULL")
	}
}

// T_RC07: List が指定 profile+resource のエントリをすべて返す
func TestList_All(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entries := []Entry{
		makeEntry("default", "clients", "1", `{"id":1}`),
		makeEntry("default", "clients", "2", `{"id":2}`),
		makeEntry("default", "clients", "3", `{"id":3}`),
	}
	for _, e := range entries {
		if err := rc.Upsert(ctx, e); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}

	list, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len(list): got %d, want 3", len(list))
	}
}

// T_RC08: List が別 resource のエントリを除外する
func TestList_FilterByResource(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	if err := rc.Upsert(ctx, makeEntry("default", "clients", "1", `{"id":1}`)); err != nil {
		t.Fatalf("Upsert clients: %v", err)
	}
	if err := rc.Upsert(ctx, makeEntry("default", "projects", "1", `{"id":1}`)); err != nil {
		t.Fatalf("Upsert projects: %v", err)
	}

	list, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(list): got %d, want 1", len(list))
	}
}

// T_RC09: List が空のときに空スライスを返す
func TestList_Empty(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	list, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Error("List should return empty slice, not nil")
	}
	if len(list) != 0 {
		t.Errorf("len(list): got %d, want 0", len(list))
	}
}

// T_RC10: Delete が指定エントリを削除する
func TestDelete(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entry := makeEntry("default", "clients", "1", `{"id":1}`)
	if err := rc.Upsert(ctx, entry); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := rc.Delete(ctx, entry.Key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := rc.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("entry should be deleted")
	}
}

// T_RC11: Delete が存在しないキーに対してエラーなし
func TestDelete_NotFound(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	err := rc.Delete(ctx, NewEntityKey("default", "clients", "nonexistent"))
	if err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

// T_RC12: DeleteAll が指定 profile+resource のエントリを全削除する
func TestDeleteAll(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	for i := range []int{1, 2, 3} {
		e := makeEntry("default", "clients", string(rune('1'+i)), `{"id":0}`)
		if err := rc.Upsert(ctx, e); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}

	if err := rc.DeleteAll(ctx, "default", "clients"); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	list, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("after DeleteAll, len(list): got %d, want 0", len(list))
	}
}

// T_RC13: UpsertMany が複数エントリをトランザクションで挿入する
func TestUpsertMany(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	entries := []Entry{
		makeEntry("default", "clients", "1", `{"id":1}`),
		makeEntry("default", "clients", "2", `{"id":2}`),
		makeEntry("default", "clients", "3", `{"id":3}`),
	}
	if err := rc.UpsertMany(ctx, entries); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	list, err := rc.List(ctx, "default", "clients")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len(list): got %d, want 3", len(list))
	}
}

// T_RC14: UpsertMany がエラー時にロールバックする
// entity_id に空文字列（NOT NULL 制約は満たすが PRIMARY KEY 制約は満たす）は問題ない。
// ここでは payload_json に NULL を直接 INSERT して NOT NULL 制約違反を起こす。
func TestUpsertMany_RollbackOnError(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	// 1件目は正常
	good := makeEntry("default", "clients", "1", `{"id":1}`)
	// 2件目: entity_id が空文字（NOT NULL を満たすが空文字）→ PRIMARY KEY として有効
	// payload_json に nil をセットして NOT NULL 制約違反を起こす
	bad := Entry{
		Key:         NewEntityKey("default", "clients", ""),
		PayloadJSON: nil, // NOT NULL 違反
		UpdatedAt:   sql.NullString{Valid: false},
	}

	err := rc.UpsertMany(ctx, []Entry{good, bad})
	if err == nil {
		t.Fatal("UpsertMany should return error for nil PayloadJSON")
	}

	// ロールバックされているので good も挿入されていないはず
	got, err2 := rc.Get(ctx, good.Key)
	if err2 != nil {
		t.Fatalf("Get: %v", err2)
	}
	if got != nil {
		t.Error("transaction should have been rolled back, but entry was found")
	}
}

// T_RC15: UpsertMany が空スライスに対してエラーなし
func TestUpsertMany_Empty(t *testing.T) {
	db := openTestDB(t)
	rc := NewResourceCache(db)
	ctx := context.Background()

	if err := rc.UpsertMany(ctx, []Entry{}); err != nil {
		t.Fatalf("UpsertMany empty: %v", err)
	}
}
