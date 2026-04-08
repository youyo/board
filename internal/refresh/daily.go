package refresh

import "time"

// TodayInTZ は now を tz の timezone で解釈した日付を "YYYY-MM-DD" 形式で返す。
func TodayInTZ(now time.Time, tz *time.Location) string {
	return now.In(tz).Format("2006-01-02")
}
