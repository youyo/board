package boardapi

import "net/http"

// applyAuthHeaders は BOARD API の認証ヘッダを付与する。
//
//   x-api-key: <APIKey>
//   Authorization: Bearer <APIToken>
func applyAuthHeaders(req *http.Request, apiKey, apiToken string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiToken)
}
