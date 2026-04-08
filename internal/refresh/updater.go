package refresh

import (
	"context"
	"database/sql"
	"time"

	"github.com/youyo/board/internal/cache"
)

// Updater は sync_state の各フィールドを更新するヘルパー。
// Refresher に埋め込まず、独立した型として定義する。
type Updater struct {
	syncStore *cache.SyncStateStore
}

// NewUpdater は Updater を生成する。
func NewUpdater(ss *cache.SyncStateStore) *Updater {
	return &Updater{syncStore: ss}
}

// nullString は文字列を sql.NullString に変換するヘルパー。
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// nullStringOrKeep は newVal が空の場合に existing を返し、そうでなければ newVal を返すヘルパー。
func nullStringOrKeep(newVal string, existing sql.NullString) sql.NullString {
	if newVal == "" {
		return existing
	}
	return nullString(newVal)
}

// getOrInit は SyncStateStore から state を取得し、存在しない場合は初期値を返す。
func getOrInit(ctx context.Context, ss *cache.SyncStateStore, profile, resource string) (*cache.SyncState, error) {
	state, err := ss.Get(ctx, profile, resource)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = &cache.SyncState{
			ProfileName:  profile,
			ResourceName: resource,
		}
	}
	return state, nil
}

// MarkDeltaSuccess は差分取得成功後に sync_state を更新する。
//
// 更新フィールド:
//   - last_synced_at = now (RFC3339)
//   - last_sync_mode = "delta"
//   - last_sync_status = "success"
//   - last_daily_refresh_date = TodayInTZ(now, tz)
//   - cursor_updated_at = newCursor（空の場合は既存値を保持）
//   - consecutive_failures = 0
//   - last_error_at, last_error_code, last_error_message は変更しない
func (u *Updater) MarkDeltaSuccess(
	ctx context.Context,
	profile, resource string,
	newCursor string,
	now time.Time,
	tz *time.Location,
) error {
	state, err := getOrInit(ctx, u.syncStore, profile, resource)
	if err != nil {
		return err
	}

	state.LastSyncedAt = nullString(now.UTC().Format(time.RFC3339))
	state.LastSyncMode = nullString("delta")
	state.LastSyncStatus = nullString("success")
	state.LastDailyRefreshDate = nullString(TodayInTZ(now, tz))
	state.CursorUpdatedAt = nullStringOrKeep(newCursor, state.CursorUpdatedAt)
	state.ConsecutiveFailures = 0

	return u.syncStore.Upsert(ctx, *state)
}

// MarkForceSuccess は全件取得成功後に sync_state を更新する。
//
// 更新フィールド:
//   - last_synced_at = now (RFC3339)
//   - last_full_synced_at = now (RFC3339)
//   - last_sync_mode = "full"
//   - last_sync_status = "success"
//   - last_daily_refresh_date = TodayInTZ(now, tz)
//   - cursor_updated_at = NULL（full 後はカーソルリセット）
//   - must_full_resync = false
//   - consecutive_failures = 0
func (u *Updater) MarkForceSuccess(
	ctx context.Context,
	profile, resource string,
	now time.Time,
	tz *time.Location,
) error {
	state, err := getOrInit(ctx, u.syncStore, profile, resource)
	if err != nil {
		return err
	}

	nowStr := now.UTC().Format(time.RFC3339)
	state.LastSyncedAt = nullString(nowStr)
	state.LastFullSyncedAt = nullString(nowStr)
	state.LastSyncMode = nullString("full")
	state.LastSyncStatus = nullString("success")
	state.LastDailyRefreshDate = nullString(TodayInTZ(now, tz))
	state.CursorUpdatedAt = sql.NullString{Valid: false}
	state.MustFullResync = false
	state.ConsecutiveFailures = 0

	return u.syncStore.Upsert(ctx, *state)
}

// MarkError は refresh 失敗時に sync_state を更新する。
//
// 更新フィールド:
//   - last_sync_status = "error"
//   - last_error_at = now (RFC3339)
//   - last_error_code = errCode
//   - last_error_message = errMsg
//   - consecutive_failures++
func (u *Updater) MarkError(
	ctx context.Context,
	profile, resource string,
	errCode, errMsg string,
	now time.Time,
) error {
	state, err := getOrInit(ctx, u.syncStore, profile, resource)
	if err != nil {
		return err
	}

	state.LastSyncStatus = nullString("error")
	state.LastErrorAt = nullString(now.UTC().Format(time.RFC3339))
	state.LastErrorCode = nullString(errCode)
	state.LastErrorMessage = nullString(errMsg)
	state.ConsecutiveFailures++

	return u.syncStore.Upsert(ctx, *state)
}
