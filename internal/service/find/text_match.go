package find

import (
	"strings"

	"github.com/youyo/board/internal/boardapi"
)

// containsText は fields のいずれかが text を大文字小文字を区別せず含む場合 true を返す。
// text は TrimSpace を適用してから比較する（空文字は常に false）。
func containsText(text string, fields ...string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lower) {
			return true
		}
	}
	return false
}

// derefString は *string を安全に string へ変換する。nil の場合は "" を返す。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// projectClientID は ProjectEntity の nested Client から client ID を取得する。
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
