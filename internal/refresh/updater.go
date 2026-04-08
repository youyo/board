package refresh

import (
	"context"
	"database/sql"
	"time"

	"github.com/youyo/board/internal/cache"
)

// Updater is a helper for updating individual fields in sync_state.
// Defined as an independent type rather than embedded in Refresher.
type Updater struct {
	syncStore *cache.SyncStateStore
}

// NewUpdater creates an Updater.
func NewUpdater(ss *cache.SyncStateStore) *Updater {
	return &Updater{syncStore: ss}
}

// nullString is a helper that converts a string to sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// nullStringOrKeep is a helper that returns existing if newVal is empty, otherwise returns newVal.
func nullStringOrKeep(newVal string, existing sql.NullString) sql.NullString {
	if newVal == "" {
		return existing
	}
	return nullString(newVal)
}

// getOrInit retrieves state from SyncStateStore, returning an initial value if not found.
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

// MarkDeltaSuccess updates sync_state after a successful delta fetch.
//
// Updated fields:
//   - last_synced_at = now (RFC3339)
//   - last_sync_mode = "delta"
//   - last_sync_status = "success"
//   - last_daily_refresh_date = TodayInTZ(now, tz)
//   - cursor_updated_at = newCursor (retain existing if empty)
//   - consecutive_failures = 0
//   - last_error_at, last_error_code, last_error_message are not changed
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

// MarkForceSuccess updates sync_state after a successful full fetch.
//
// Updated fields:
//   - last_synced_at = now (RFC3339)
//   - last_full_synced_at = now (RFC3339)
//   - last_sync_mode = "full"
//   - last_sync_status = "success"
//   - last_daily_refresh_date = TodayInTZ(now, tz)
//   - cursor_updated_at = NULL (reset cursor after full fetch)
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

// MarkLockAcquired sets refresh_started_at and refresh_owner.
// Creates a new record (upsert) if sync_state does not exist.
func (u *Updater) MarkLockAcquired(ctx context.Context, profile, resource, ownerID string, now time.Time) error {
	state, err := getOrInit(ctx, u.syncStore, profile, resource)
	if err != nil {
		return err
	}

	state.RefreshStartedAt = nullString(now.UTC().Format(time.RFC3339))
	state.RefreshOwner = nullString(ownerID)

	return u.syncStore.Upsert(ctx, *state)
}

// MarkLockReleased resets refresh_started_at and refresh_owner to NULL.
// Does nothing if sync_state does not exist (no error).
func (u *Updater) MarkLockReleased(ctx context.Context, profile, resource string) error {
	state, err := u.syncStore.Get(ctx, profile, resource)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}

	state.RefreshStartedAt = sql.NullString{Valid: false}
	state.RefreshOwner = sql.NullString{Valid: false}

	return u.syncStore.Upsert(ctx, *state)
}

// MarkError updates sync_state on refresh failure.
//
// Updated fields:
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
