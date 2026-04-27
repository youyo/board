package find

import "fmt"

// UI base URL の導出は呼び出し元（app.go）で行う:
// "://api." を "://" に置換する単純規則。BOARD_UI_BASE_URL 環境変数で上書き可能。

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

// clientURL は clients/{id}/edit の編集ページ URL を返す。
func clientURL(uiBase string, id int) string {
	if id == 0 || uiBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/clients/%d/edit", uiBase, id)
}

// vendorURL は payees/{id}/edit の編集ページ URL を返す（BOARD UI は /payees）。
func vendorURL(uiBase string, id int) string {
	if id == 0 || uiBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/payees/%d/edit", uiBase, id)
}
