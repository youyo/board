package cache

import (
	"context"
	"database/sql"
	"fmt"
)

// SyncState は sync_state テーブルの1行を表す。
type SyncState struct {
	ProfileName          string
	ResourceName         string
	LastSyncedAt         sql.NullString
	CursorUpdatedAt      sql.NullString
	LastFullSyncedAt     sql.NullString
	LastSyncMode         sql.NullString
	LastSyncStatus       sql.NullString
	LastDailyRefreshDate sql.NullString
	MustFullResync       bool
	ExpiredAt            sql.NullString
	InvalidateReason     sql.NullString
	LastErrorAt          sql.NullString
	LastErrorCode        sql.NullString
	LastErrorMessage     sql.NullString
	ConsecutiveFailures  int64
	RefreshStartedAt     sql.NullString
	RefreshOwner         sql.NullString
	CacheVersion         int64
	SchemaVersion        int64
}

// SyncStateStore は sync_state テーブルの CRUD 操作を提供する。
type SyncStateStore struct {
	db *DB
}

// NewSyncStateStore は SyncStateStore を生成する。
func NewSyncStateStore(db *DB) *SyncStateStore {
	return &SyncStateStore{db: db}
}

const sqlSyncStateGet = `
SELECT
  profile_name, resource_name,
  last_synced_at, cursor_updated_at, last_full_synced_at,
  last_sync_mode, last_sync_status, last_daily_refresh_date,
  must_full_resync, expired_at, invalidate_reason,
  last_error_at, last_error_code, last_error_message,
  consecutive_failures, refresh_started_at, refresh_owner,
  cache_version, schema_version
FROM sync_state
WHERE profile_name = ? AND resource_name = ?`

const sqlSyncStateUpsert = `
INSERT OR REPLACE INTO sync_state (
  profile_name, resource_name,
  last_synced_at, cursor_updated_at, last_full_synced_at,
  last_sync_mode, last_sync_status, last_daily_refresh_date,
  must_full_resync, expired_at, invalidate_reason,
  last_error_at, last_error_code, last_error_message,
  consecutive_failures, refresh_started_at, refresh_owner,
  cache_version, schema_version
) VALUES (
  ?, ?,
  ?, ?, ?,
  ?, ?, ?,
  ?, ?, ?,
  ?, ?, ?,
  ?, ?, ?,
  ?, ?
)`

const sqlSyncStateDelete = `
DELETE FROM sync_state
WHERE profile_name = ? AND resource_name = ?`

// Get は指定 profile+resource の SyncState を返す。存在しない場合は nil, nil を返す。
func (s *SyncStateStore) Get(ctx context.Context, profile, resource string) (*SyncState, error) {
	row := s.db.db.QueryRowContext(ctx, sqlSyncStateGet, profile, resource)

	var st SyncState
	var mustFullResync int64
	err := row.Scan(
		&st.ProfileName, &st.ResourceName,
		&st.LastSyncedAt, &st.CursorUpdatedAt, &st.LastFullSyncedAt,
		&st.LastSyncMode, &st.LastSyncStatus, &st.LastDailyRefreshDate,
		&mustFullResync, &st.ExpiredAt, &st.InvalidateReason,
		&st.LastErrorAt, &st.LastErrorCode, &st.LastErrorMessage,
		&st.ConsecutiveFailures, &st.RefreshStartedAt, &st.RefreshOwner,
		&st.CacheVersion, &st.SchemaVersion,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sync_state: get: %w", err)
	}
	st.MustFullResync = mustFullResync != 0
	return &st, nil
}

// Upsert は SyncState を挿入または上書きする（全 19 カラム）。
func (s *SyncStateStore) Upsert(ctx context.Context, state SyncState) error {
	mustFullResync := int64(0)
	if state.MustFullResync {
		mustFullResync = 1
	}
	_, err := s.db.db.ExecContext(ctx, sqlSyncStateUpsert,
		state.ProfileName, state.ResourceName,
		state.LastSyncedAt, state.CursorUpdatedAt, state.LastFullSyncedAt,
		state.LastSyncMode, state.LastSyncStatus, state.LastDailyRefreshDate,
		mustFullResync, state.ExpiredAt, state.InvalidateReason,
		state.LastErrorAt, state.LastErrorCode, state.LastErrorMessage,
		state.ConsecutiveFailures, state.RefreshStartedAt, state.RefreshOwner,
		state.CacheVersion, state.SchemaVersion,
	)
	if err != nil {
		return fmt.Errorf("sync_state: upsert: %w", err)
	}
	return nil
}

// Delete は指定 profile+resource の SyncState を削除する。存在しない場合もエラーなし。
func (s *SyncStateStore) Delete(ctx context.Context, profile, resource string) error {
	_, err := s.db.db.ExecContext(ctx, sqlSyncStateDelete, profile, resource)
	if err != nil {
		return fmt.Errorf("sync_state: delete: %w", err)
	}
	return nil
}
