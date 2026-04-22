package find

import (
	"strings"

	"github.com/youyo/board/internal/boardapi"
)

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

// derefString は *string を安全に string に変換する。nil の場合は "" を返す。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// projectClientID は ProjectEntity の nested Client から client ID を取得する。
// M44: ClientID フィールド廃止に伴い Client.ID 参照に統合。
// Client が nil の場合は 0 を返す。
func projectClientID(p boardapi.ProjectEntity) int {
	if p.Client == nil {
		return 0
	}
	return p.Client.ID
}

// projectClientIDPtr は *ProjectEntity の nested Client から client ID を取得する。
// nil の ProjectEntity の場合は 0 を返す。
func projectClientIDPtr(p *boardapi.ProjectEntity) int {
	if p == nil || p.Client == nil {
		return 0
	}
	return p.Client.ID
}
