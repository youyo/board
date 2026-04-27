package find

import (
	"fmt"
	"strings"
)

// deriveUIBaseURL は API base URL から UI base URL を導出する。
// 例: "https://api.the-board.jp" → "https://the-board.jp"
//
// "://api." を "://" に置換する単純規則。Sandbox 等で API host が
// "api." prefix を持たない構成では BOARD_UI_BASE_URL 環境変数で上書きする
// （BuildUIBaseURL 参照）。
func deriveUIBaseURL(apiBase string) string {
	return strings.Replace(apiBase, "://api.", "://", 1)
}

// projectURL は projects/{id}/edit の編集ページ URL を返す。id=0 のときは空文字。
func projectURL(uiBase string, id int) string {
	if id == 0 || uiBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/projects/%d/edit", uiBase, id)
}

// documentURL は documents/{id}/edit の編集ページ URL を返す。
// estimate / order / delivery / receipt / invoice / purchase_order の各 entity ID に使用する。
func documentURL(uiBase string, id int) string {
	if id == 0 || uiBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/documents/%d/edit", uiBase, id)
}
