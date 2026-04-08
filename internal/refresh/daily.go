package refresh

import "time"

// TodayInTZ returns the date of now interpreted in the tz timezone, formatted as "YYYY-MM-DD".
func TodayInTZ(now time.Time, tz *time.Location) string {
	return now.In(tz).Format("2006-01-02")
}
