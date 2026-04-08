package cache

import (
	"context"
	"database/sql"
	"fmt"
)

// CacheMetaStore is a simple KV store providing CRUD operations for the cache_meta table.
type CacheMetaStore struct {
	db *DB
}

// NewCacheMetaStore creates a CacheMetaStore.
func NewCacheMetaStore(db *DB) *CacheMetaStore {
	return &CacheMetaStore{db: db}
}

const sqlCacheMetaGet = `
SELECT value FROM cache_meta WHERE key = ?`

const sqlCacheMetaSet = `
INSERT OR REPLACE INTO cache_meta (key, value) VALUES (?, ?)`

const sqlCacheMetaDelete = `
DELETE FROM cache_meta WHERE key = ?`

// Get returns the value for the specified key. Returns "", nil if not found.
func (s *CacheMetaStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.db.QueryRowContext(ctx, sqlCacheMetaGet, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cache_meta: get: %w", err)
	}
	return value, nil
}

// Set inserts or overwrites a key-value pair.
func (s *CacheMetaStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.db.ExecContext(ctx, sqlCacheMetaSet, key, value)
	if err != nil {
		return fmt.Errorf("cache_meta: set: %w", err)
	}
	return nil
}

// Delete deletes the entry for the specified key. No error if not found.
func (s *CacheMetaStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.db.ExecContext(ctx, sqlCacheMetaDelete, key)
	if err != nil {
		return fmt.Errorf("cache_meta: delete: %w", err)
	}
	return nil
}
