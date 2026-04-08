package refresh

import (
	"time"

	"github.com/youyo/board/internal/cache"
)

// NeedsDailyRefresh returns whether a daily refresh is needed
// based on the given SyncState, current time, and timezone.
//
// Decision order:
//  1. state == nil (first time, no record) → true
//  2. state.MustFullResync == true → true
//  3. state.ExpiredAt is valid and in the past → true
//  4. state.LastDailyRefreshDate is NULL → true
//  5. TodayInTZ(now, tz) != state.LastDailyRefreshDate → true/false
//
// The DailyAutoRefresh flag (config) OFF check is the caller's responsibility.
// NeedsDailyRefresh does not accept config (to keep it a pure function).
func NeedsDailyRefresh(state *cache.SyncState, now time.Time, tz *time.Location) bool {
	if state == nil {
		return true
	}
	if state.MustFullResync {
		return true
	}
	if state.ExpiredAt.Valid {
		expiredAt, err := time.Parse(time.RFC3339, state.ExpiredAt.String)
		if err == nil && now.After(expiredAt) {
			return true
		}
	}
	if !state.LastDailyRefreshDate.Valid {
		return true
	}
	today := TodayInTZ(now, tz)
	return today != state.LastDailyRefreshDate.String
}
