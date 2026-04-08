package cache

import (
	"fmt"
	"strconv"
)

// Migrate checks the schema version and applies DDL as needed.
// Designed to be idempotent: safe to call multiple times.
func Migrate(db *DB) error {
	// first create the cache_meta table (needed to check version)
	if _, err := db.db.Exec(ddlCacheMeta); err != nil {
		return fmt.Errorf("cache: migrate: create cache_meta: %w", err)
	}

	// get current version
	current := 0
	var val string
	err := db.db.QueryRow("SELECT value FROM cache_meta WHERE key = 'db_schema_version'").Scan(&val)
	if err == nil {
		current, _ = strconv.Atoi(val)
	}

	if current >= schemaVersion {
		return nil // already up to date
	}

	// apply migrations
	migrations := []struct {
		version int
		ddl     string
	}{
		{1, ddlResourceCache + ddlSyncState},
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("cache: migrate v%d: %w", m.version, err)
		}
	}

	// update version
	_, err = db.db.Exec(
		"INSERT OR REPLACE INTO cache_meta (key, value) VALUES ('db_schema_version', ?)",
		strconv.Itoa(schemaVersion),
	)
	if err != nil {
		return fmt.Errorf("cache: migrate: update version: %w", err)
	}

	return nil
}
