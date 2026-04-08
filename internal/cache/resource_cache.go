package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Entry は resource_cache テーブルの1行を表す。
type Entry struct {
	Key         EntityKey
	PayloadJSON json.RawMessage
	// UpdatedAt は BOARD API の updated_at に対応する。NULL 許容。
	// SQLite では NULL は最小値として扱われ、ORDER BY updated_at ASC では先頭に来る。
	UpdatedAt sql.NullString
	FetchedAt string
}

// ResourceCache は resource_cache テーブルの CRUD 操作を提供する。
type ResourceCache struct {
	db *DB
}

// NewResourceCache は ResourceCache を生成する。
func NewResourceCache(db *DB) *ResourceCache {
	return &ResourceCache{db: db}
}

const sqlUpsert = `
INSERT OR REPLACE INTO resource_cache
  (profile_name, resource_name, entity_id, payload_json, updated_at, fetched_at)
VALUES (?, ?, ?, ?, ?, ?)`

const sqlGet = `
SELECT profile_name, resource_name, entity_id, payload_json, updated_at, fetched_at
FROM resource_cache
WHERE profile_name = ? AND resource_name = ? AND entity_id = ?`

const sqlList = `
SELECT profile_name, resource_name, entity_id, payload_json, updated_at, fetched_at
FROM resource_cache
WHERE profile_name = ? AND resource_name = ?
ORDER BY updated_at ASC`

const sqlDelete = `
DELETE FROM resource_cache
WHERE profile_name = ? AND resource_name = ? AND entity_id = ?`

const sqlDeleteAll = `
DELETE FROM resource_cache
WHERE profile_name = ? AND resource_name = ?`

// Upsert はエントリを挿入または上書きする。
// FetchedAt は現在時刻（UTC, RFC3339）で自動設定される。
func (rc *ResourceCache) Upsert(ctx context.Context, entry Entry) error {
	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	_, err := rc.db.db.ExecContext(ctx, sqlUpsert,
		entry.Key.Profile,
		entry.Key.Resource,
		entry.Key.EntityID,
		entry.PayloadJSON,
		entry.UpdatedAt,
		fetchedAt,
	)
	if err != nil {
		return fmt.Errorf("cache: upsert: %w", err)
	}
	return nil
}

// UpsertMany は複数エントリをトランザクションで一括挿入/更新する。
// いずれかのエントリでエラーが発生した場合はロールバックする。
func (rc *ResourceCache) UpsertMany(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := rc.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cache: upsert_many: begin: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, sqlUpsert)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("cache: upsert_many: prepare: %w", err)
	}

	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	for _, entry := range entries {
		_, err := stmt.ExecContext(ctx,
			entry.Key.Profile,
			entry.Key.Resource,
			entry.Key.EntityID,
			entry.PayloadJSON,
			entry.UpdatedAt,
			fetchedAt,
		)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			stmt.Close()
			return fmt.Errorf("cache: upsert_many: exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		stmt.Close()
		return fmt.Errorf("cache: upsert_many: commit: %w", err)
	}
	// stmt.Close() は tx.Commit() の後に呼ぶ
	stmt.Close()
	return nil
}

// Get は指定キーのエントリを返す。存在しない場合は nil, nil を返す。
func (rc *ResourceCache) Get(ctx context.Context, key EntityKey) (*Entry, error) {
	row := rc.db.db.QueryRowContext(ctx, sqlGet,
		key.Profile, key.Resource, key.EntityID,
	)

	var e Entry
	err := row.Scan(
		&e.Key.Profile,
		&e.Key.Resource,
		&e.Key.EntityID,
		&e.PayloadJSON,
		&e.UpdatedAt,
		&e.FetchedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache: get: %w", err)
	}
	return &e, nil
}

// List は指定 profile+resource のエントリを updated_at ASC 順で返す。
// SQLite では NULL は最小値として扱われるため、updated_at が NULL のエントリが先頭に来る。
func (rc *ResourceCache) List(ctx context.Context, profile, resource string) ([]Entry, error) {
	rows, err := rc.db.db.QueryContext(ctx, sqlList, profile, resource)
	if err != nil {
		return nil, fmt.Errorf("cache: list: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(
			&e.Key.Profile,
			&e.Key.Resource,
			&e.Key.EntityID,
			&e.PayloadJSON,
			&e.UpdatedAt,
			&e.FetchedAt,
		); err != nil {
			return nil, fmt.Errorf("cache: list: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cache: list: rows: %w", err)
	}
	return entries, nil
}

// Delete は指定キーのエントリを削除する。存在しない場合もエラーなし。
func (rc *ResourceCache) Delete(ctx context.Context, key EntityKey) error {
	_, err := rc.db.db.ExecContext(ctx, sqlDelete,
		key.Profile, key.Resource, key.EntityID,
	)
	if err != nil {
		return fmt.Errorf("cache: delete: %w", err)
	}
	return nil
}

// DeleteAll は指定 profile+resource のエントリをすべて削除する。
func (rc *ResourceCache) DeleteAll(ctx context.Context, profile, resource string) error {
	_, err := rc.db.db.ExecContext(ctx, sqlDeleteAll, profile, resource)
	if err != nil {
		return fmt.Errorf("cache: delete_all: %w", err)
	}
	return nil
}
