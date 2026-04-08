package refresh

import (
	"database/sql"
	"testing"
	"time"

	"github.com/youyo/board/internal/cache"
)

// makeState はテスト用の SyncState を生成するヘルパー。
func makeState() *cache.SyncState {
	return &cache.SyncState{
		ProfileName:  "default",
		ResourceName: "clients",
	}
}

// mustLoadTZ は timezone ロードのヘルパー。失敗時は t.Fatal。
func mustLoadTZ(t *testing.T, name string) *time.Location {
	t.Helper()
	tz, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) failed: %v", name, err)
	}
	return tz
}

func TestNeedsDailyRefresh(t *testing.T) {
	// 基準時刻: 2026-01-15 12:00:00 UTC
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	utc := mustLoadTZ(t, "UTC")
	jst := mustLoadTZ(t, "Asia/Tokyo")

	tests := []struct {
		name     string
		state    *cache.SyncState
		now      time.Time
		tz       *time.Location
		expected bool
	}{
		{
			// T_NR01: state が nil → true（初回）
			name:     "T_NR01_nil_state",
			state:    nil,
			now:      now,
			tz:       utc,
			expected: true,
		},
		{
			// T_NR02: MustFullResync が true → true
			name: "T_NR02_must_full_resync",
			state: func() *cache.SyncState {
				s := makeState()
				s.MustFullResync = true
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: true,
		},
		{
			// T_NR03: ExpiredAt が過去 → true
			name: "T_NR03_expired_at_past",
			state: func() *cache.SyncState {
				s := makeState()
				s.ExpiredAt = sql.NullString{String: "2020-01-01T00:00:00Z", Valid: true}
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: true,
		},
		{
			// T_NR04: ExpiredAt が未来 + 今日実行済み → false
			name: "T_NR04_expired_at_future_today_done",
			state: func() *cache.SyncState {
				s := makeState()
				s.ExpiredAt = sql.NullString{String: "2030-01-01T00:00:00Z", Valid: true}
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: false,
		},
		{
			// T_NR05: LastDailyRefreshDate が NULL → true
			name: "T_NR05_last_daily_refresh_null",
			state: func() *cache.SyncState {
				s := makeState()
				s.LastDailyRefreshDate = sql.NullString{Valid: false}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: true,
		},
		{
			// T_NR06: LastDailyRefreshDate が昨日 → true
			name: "T_NR06_last_daily_refresh_yesterday",
			state: func() *cache.SyncState {
				s := makeState()
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-14", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: true,
		},
		{
			// T_NR07: LastDailyRefreshDate が今日 → false
			name: "T_NR07_last_daily_refresh_today",
			state: func() *cache.SyncState {
				s := makeState()
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: false,
		},
		{
			// T_NR08: JST 跨ぎ：UTC 01:00 は JST 10:00（当日）、LastDailyRefreshDate="2026-01-15" → false
			name: "T_NR08_jst_same_day",
			state: func() *cache.SyncState {
				s := makeState()
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      time.Date(2026, 1, 15, 1, 0, 0, 0, time.UTC),
			tz:       jst,
			expected: false,
		},
		{
			// T_NR09: JST 跨ぎ：UTC 15:00 は JST 翌日 00:00、LastDailyRefreshDate="2026-01-15" → true
			name: "T_NR09_jst_next_day",
			state: func() *cache.SyncState {
				s := makeState()
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
			tz:       jst,
			expected: true,
		},
		{
			// T_NR10: ExpiredAt パース失敗は無視 + 今日実行済み → false
			name: "T_NR10_expired_at_parse_failure",
			state: func() *cache.SyncState {
				s := makeState()
				s.ExpiredAt = sql.NullString{String: "invalid-date", Valid: true}
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: false,
		},
		{
			// T_NR11: MustFullResync が false かつ今日実行済み（正常状態）→ false
			name: "T_NR11_normal_state",
			state: func() *cache.SyncState {
				s := makeState()
				s.MustFullResync = false
				s.LastDailyRefreshDate = sql.NullString{String: "2026-01-15", Valid: true}
				return s
			}(),
			now:      now,
			tz:       utc,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsDailyRefresh(tt.state, tt.now, tt.tz)
			if got != tt.expected {
				t.Errorf("NeedsDailyRefresh() = %v, want %v", got, tt.expected)
			}
		})
	}
}
