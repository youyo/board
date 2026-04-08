package cache

import (
	"context"
	"database/sql"
	"fmt"
)

// CacheMetaStore は cache_meta テーブルの CRUD 操作を提供するシンプルな KV ストア。
type CacheMetaStore struct {
	db *DB
}

// NewCacheMetaStore は CacheMetaStore を生成する。
func NewCacheMetaStore(db *DB) *CacheMetaStore {
	return &CacheMetaStore{db: db}
}

const sqlCacheMetaGet = `
SELECT value FROM cache_meta WHERE key = ?`

const sqlCacheMetaSet = `
INSERT OR REPLACE INTO cache_meta (key, value) VALUES (?, ?)`

const sqlCacheMetaDelete = `
DELETE FROM cache_meta WHERE key = ?`

// Get は指定キーの値を返す。存在しない場合は "", nil を返す。
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

// Set はキーと値を挿入または上書きする。
func (s *CacheMetaStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.db.ExecContext(ctx, sqlCacheMetaSet, key, value)
	if err != nil {
		return fmt.Errorf("cache_meta: set: %w", err)
	}
	return nil
}

// Delete は指定キーのエントリを削除する。存在しない場合もエラーなし。
func (s *CacheMetaStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.db.ExecContext(ctx, sqlCacheMetaDelete, key)
	if err != nil {
		return fmt.Errorf("cache_meta: delete: %w", err)
	}
	return nil
}
