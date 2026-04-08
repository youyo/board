package cache

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB manages a SQLite database connection.
type DB struct {
	db *sql.DB
}

// Open opens a SQLite database and configures PRAGMA settings.
// If dsn is ":memory:", an in-memory DB is used.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cache: open: %w", err)
	}
	// configure PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("cache: pragma: %w", err)
		}
	}
	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SQLDB returns the internal *sql.DB.
// Used by upper-level packages (e.g., repository) to execute queries.
func (d *DB) SQLDB() *sql.DB {
	return d.db
}
