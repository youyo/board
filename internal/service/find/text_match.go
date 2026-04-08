package find

import "strings"

// containsText returns true if any of the given fields contain the text
// as a case-insensitive substring.
func containsText(text string, fields ...string) bool {
	lower := strings.ToLower(text)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lower) {
			return true
		}
	}
	return false
}
