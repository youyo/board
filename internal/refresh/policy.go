package refresh

import (
	"time"

	"github.com/youyo/board/internal/cache"
)

// NeedsDailyRefresh は、指定された SyncState と現在時刻・timezone を元に
// daily refresh が必要かどうかを返す。
//
// 判定順:
//  1. state == nil（初回、レコードなし）→ true
//  2. state.MustFullResync == true → true
//  3. state.ExpiredAt が有効かつ now より過去 → true
//  4. state.LastDailyRefreshDate が NULL → true
//  5. TodayInTZ(now, tz) != state.LastDailyRefreshDate → true/false
//
// DailyAutoRefresh フラグ（config）の OFF 判定は呼び出し側の責務。
// NeedsDailyRefresh は config を受け取らない（純粋関数として保つため）。
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
