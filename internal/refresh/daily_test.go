package refresh

import (
	"testing"
	"time"
)

func TestTodayInTZ(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		tzName   string
		expected string
	}{
		{
			// T_DR01: get date in UTC
			name:     "T_DR01_UTC",
			now:      time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			tzName:   "UTC",
			expected: "2026-01-15",
		},
		{
			// T_DR02: JST (+09:00) date differs from UTC (UTC 01:00 is JST 10:00 → same day)
			name:     "T_DR02_JST_same_day",
			now:      time.Date(2026, 1, 15, 1, 0, 0, 0, time.UTC),
			tzName:   "Asia/Tokyo",
			expected: "2026-01-15",
		},
		{
			// T_DR03: crossing midnight JST (UTC 15:00 is JST next day 00:00)
			name:     "T_DR03_JST_next_day",
			now:      time.Date(2026, 1, 15, 15, 0, 0, 0, time.UTC),
			tzName:   "Asia/Tokyo",
			expected: "2026-01-16",
		},
		{
			// T_DR04: America/New_York (UTC-4 DST) UTC 23:00 is New York 19:00 → same day
			name:     "T_DR04_America_New_York",
			now:      time.Date(2026, 6, 1, 23, 0, 0, 0, time.UTC),
			tzName:   "America/New_York",
			expected: "2026-06-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tz, err := time.LoadLocation(tt.tzName)
			if err != nil {
				t.Fatalf("LoadLocation(%q) failed: %v", tt.tzName, err)
			}
			got := TodayInTZ(tt.now, tz)
			if got != tt.expected {
				t.Errorf("TodayInTZ(%v, %q) = %q, want %q", tt.now, tt.tzName, got, tt.expected)
			}
		})
	}
}
