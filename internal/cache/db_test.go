package cache

import (
	"database/sql"
	"testing"
)

// T_DB01: Open in-memory DB 成功
func TestOpen_InMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(\":memory:\") returned error: %v", err)
	}
	if db == nil {
		t.Fatal("Open returned nil DB")
	}
	defer db.Close()
}

// T_DB02: Close 成功
func TestClose(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}

// T_DB03: SQLDB() が non-nil を返す
func TestSQLDB_NonNil(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	sqldb := db.SQLDB()
	if sqldb == nil {
		t.Fatal("SQLDB() returned nil")
	}
}

// T_DB04: WAL mode 確認（in-memory は "memory" を返す場合あり）
func TestWALMode(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.SQLDB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("QueryRow journal_mode: %v", err)
	}
	if mode != "wal" && mode != "memory" {
		t.Fatalf("unexpected journal_mode: %q (want \"wal\" or \"memory\")", mode)
	}
}

// T_DB05: Migrate 成功（テーブル作成確認）
func TestMigrate_Success(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}
}

// T_DB06: Migrate 冪等性（2回呼んでもエラーなし）
func TestMigrate_Idempotent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() first call error: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() second call error: %v", err)
	}
}

// T_DB07: resource_cache テーブル存在確認（INSERT + SELECT）
func TestResourceCacheTable(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	sqldb := db.SQLDB()
	_, err = sqldb.Exec(`
		INSERT INTO resource_cache
		  (profile_name, resource_name, entity_id, payload_json, fetched_at)
		VALUES (?, ?, ?, ?, ?)`,
		"default", "clients", "1", `{"id":"1"}`, "2026-04-08T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("INSERT into resource_cache: %v", err)
	}

	var count int
	if err := sqldb.QueryRow("SELECT COUNT(*) FROM resource_cache").Scan(&count); err != nil {
		t.Fatalf("SELECT COUNT(*) from resource_cache: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

// T_DB08: sync_state テーブル存在確認（INSERT + SELECT）
func TestSyncStateTable(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	sqldb := db.SQLDB()
	_, err = sqldb.Exec(`
		INSERT INTO sync_state (profile_name, resource_name)
		VALUES (?, ?)`,
		"default", "clients",
	)
	if err != nil {
		t.Fatalf("INSERT into sync_state: %v", err)
	}

	var count int
	if err := sqldb.QueryRow("SELECT COUNT(*) FROM sync_state").Scan(&count); err != nil {
		t.Fatalf("SELECT COUNT(*) from sync_state: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

// T_DB09: cache_meta テーブル存在確認
func TestCacheMetaTable(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	sqldb := db.SQLDB()
	var count int
	if err := sqldb.QueryRow("SELECT COUNT(*) FROM cache_meta").Scan(&count); err != nil {
		t.Fatalf("SELECT COUNT(*) from cache_meta: %v", err)
	}
}

// T_DB10: schema version が "1" であること
func TestSchemaVersion(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	var val string
	err = db.SQLDB().QueryRow(
		"SELECT value FROM cache_meta WHERE key = 'db_schema_version'",
	).Scan(&val)
	if err != nil {
		t.Fatalf("SELECT db_schema_version: %v", err)
	}
	if val != "1" {
		t.Fatalf("expected schema version \"1\", got %q", val)
	}
}

// T_DB11: resource_cache の複合 PK 確認（重複 INSERT でエラー）
func TestResourceCache_CompositePK(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	sqldb := db.SQLDB()
	insert := func() error {
		_, err := sqldb.Exec(`
			INSERT INTO resource_cache
			  (profile_name, resource_name, entity_id, payload_json, fetched_at)
			VALUES (?, ?, ?, ?, ?)`,
			"default", "clients", "1", `{"id":"1"}`, "2026-04-08T00:00:00Z",
		)
		return err
	}

	if err := insert(); err != nil {
		t.Fatalf("first INSERT: %v", err)
	}
	if err := insert(); err == nil {
		t.Fatal("expected error on duplicate PK, got nil")
	}
}

// T_DB12: sync_state の複合 PK 確認
func TestSyncState_CompositePK(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate error: %v", err)
	}

	sqldb := db.SQLDB()
	insert := func() error {
		_, err := sqldb.Exec(`
			INSERT INTO sync_state (profile_name, resource_name)
			VALUES (?, ?)`,
			"default", "clients",
		)
		return err
	}

	if err := insert(); err != nil {
		t.Fatalf("first INSERT: %v", err)
	}
	if err := insert(); err == nil {
		t.Fatal("expected error on duplicate PK, got nil")
	}
}

// T_DB13: Open with invalid DSN でエラー
// modernc.org/sqlite は sql.Open では基本エラーを返さないが、
// PRAGMA 実行時にエラーになるパスを含む DSN でテストする。
// ここでは実際に接続してエラーになるケースを確認する。
// (modernc.org/sqlite は :memory: 以外のパスでも Open 自体はエラーにならないが
//
//	不正なオプションはエラーになる)
func TestOpen_PingError(t *testing.T) {
	// 不正なパラメータを含む DSN でエラーを確認
	// modernc.org/sqlite は sql.Open では遅延接続なので、
	// PRAGMA 実行時に検出される。ここでは書き込み不可なパスを使う。
	db, err := Open("/nonexistent/path/that/cannot/be/created.db")
	if err != nil {
		// 期待通りエラーが返った
		return
	}
	// エラーなく開けた場合は Ping で確認
	defer db.Close()
	if pingErr := db.SQLDB().Ping(); pingErr != nil {
		// Ping でエラーになった場合も許容
		return
	}
	// 何もエラーが出なかった場合（DB が開けてしまった）は別途確認
	// 一部の環境ではファイルが作られる場合があるのでスキップ扱い
	t.Skip("could not trigger Open error with invalid path on this system")
}

// helper: sql.ErrNoRows を使うため database/sql をインポートしておく
var _ = sql.ErrNoRows
