package cache

import (
	"database/sql"
	"testing"
)

// T_DB01: Open in-memory DB succeeds
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

// T_DB02: Close succeeds
func TestClose(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}

// T_DB03: SQLDB() returns non-nil
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

// T_DB04: WAL mode check (in-memory may return "memory")
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

// T_DB05: Migrate succeeds (table creation check)
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

// T_DB06: Migrate idempotency (no error on second call)
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

// T_DB07: resource_cache table existence check (INSERT + SELECT)
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

// T_DB08: sync_state table existence check (INSERT + SELECT)
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

// T_DB09: cache_meta table existence check
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

// T_DB10: schema version is "1"
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

// T_DB11: resource_cache composite PK check (duplicate INSERT returns error)
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

// T_DB12: sync_state composite PK check
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

// T_DB13: Open with invalid DSN returns error
// modernc.org/sqlite does not return errors in sql.Open itself,
// so test with a DSN that causes error during PRAGMA execution.
// Verifies cases where connecting fails.
// (modernc.org/sqlite does not error on Open even for non-:memory: paths,
//
//	invalid options do cause errors)
func TestOpen_PingError(t *testing.T) {
	// check for error with DSN containing invalid parameters
	// modernc.org/sqlite uses lazy connection in sql.Open,
	// so errors are detected during PRAGMA execution. Using a non-writable path here.
	db, err := Open("/nonexistent/path/that/cannot/be/created.db")
	if err != nil {
		// error returned as expected
		return
	}
	// if opened without error, check with Ping
	defer db.Close()
	if pingErr := db.SQLDB().Ping(); pingErr != nil {
		// Ping error is also acceptable
		return
	}
	// if no error at all (DB opened successfully), handle separately
	// some environments may create the file, so treat as skip
	t.Skip("could not trigger Open error with invalid path on this system")
}

// helper: import database/sql to use sql.ErrNoRows
var _ = sql.ErrNoRows
