package find

import "github.com/youyo/board/internal/boardapi"

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
