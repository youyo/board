package boardapi

import "net/http"

// applyAuthHeaders attaches authentication headers for the BOARD API.
//
//	x-api-key: <APIKey>
//	Authorization: Bearer <APIToken>
func applyAuthHeaders(req *http.Request, apiKey, apiToken string) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiToken)
}
