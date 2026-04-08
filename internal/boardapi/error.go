// Package boardapi は BOARD API への HTTP クライアント基盤を提供する。
package boardapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// APIErrorCode はエラー種別を表す文字列定数。
type APIErrorCode string

const (
	// APIErrorUnauthorized は 401 Unauthorized を表す。
	APIErrorUnauthorized APIErrorCode = "UNAUTHORIZED"
	// APIErrorForbidden は 403 Forbidden を表す。
	APIErrorForbidden APIErrorCode = "FORBIDDEN"
	// APIErrorNotFound は 404 Not Found を表す。
	APIErrorNotFound APIErrorCode = "NOT_FOUND"
	// APIErrorRateLimit は 429 Too Many Requests を表す。
	APIErrorRateLimit APIErrorCode = "RATE_LIMIT"
	// APIErrorValidation は 400/422 バリデーションエラーを表す。
	APIErrorValidation APIErrorCode = "VALIDATION"
	// APIErrorTemporary は 5xx 一時的なサーバーエラーを表す。
	APIErrorTemporary APIErrorCode = "TEMPORARY"
	// APIErrorNetwork は transport レベルのネットワークエラーを表す。
	APIErrorNetwork APIErrorCode = "NETWORK"
	// APIErrorUnknown はその他の未分類エラーを表す。
	APIErrorUnknown APIErrorCode = "UNKNOWN"
)

// APIError は BOARD API エラーを表す。
// error インターフェースを実装する。
type APIError struct {
	Code       APIErrorCode
	StatusCode int
	Message    string
	Body       string        // 生レスポンスボディ（デバッグ用）
	RetryAfter time.Duration // Retry-After ヘッダ値（0 は未指定）
}

// Error は error インターフェースを実装する。
// APIKey/APIToken 等のシークレットは含まない。
func (e *APIError) Error() string {
	return fmt.Sprintf("boardapi error [%s] status=%d: %s", e.Code, e.StatusCode, e.Message)
}

// boardAPIErrorBody は BOARD API のエラーレスポンス JSON 構造。
type boardAPIErrorBody struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// parseError は HTTP ステータスとボディから *APIError を生成する。
func parseError(statusCode int, body []byte) *APIError {
	code := ClassifyStatusCode(statusCode)
	msg := extractMessage(body)
	return &APIError{
		Code:       code,
		StatusCode: statusCode,
		Message:    msg,
		Body:       string(body),
	}
}

// ClassifyStatusCode は HTTP ステータスを APIErrorCode にマッピングする。
// テストから参照できるよう公開している。
func ClassifyStatusCode(statusCode int) APIErrorCode {
	switch {
	case statusCode == 400:
		return APIErrorValidation
	case statusCode == 401:
		return APIErrorUnauthorized
	case statusCode == 403:
		return APIErrorForbidden
	case statusCode == 404:
		return APIErrorNotFound
	case statusCode == 422:
		return APIErrorValidation
	case statusCode == 429:
		return APIErrorRateLimit
	case statusCode >= 500 && statusCode < 600:
		return APIErrorTemporary
	default:
		return APIErrorUnknown
	}
}

// parseErrorWithHeader は HTTP レスポンスから *APIError を生成する（Retry-After対応）。
func parseErrorWithHeader(resp *http.Response, body []byte) *APIError {
	ae := parseError(resp.StatusCode, body)
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			ae.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return ae
}

// IsRetryable はエラーがリトライ対象かどうかを返す。
// 429、5xx（TEMPORARY）、ネットワークエラー（NETWORK）はリトライ対象。
// 4xx（UNAUTHORIZED/FORBIDDEN/NOT_FOUND/VALIDATION）は恒久エラーのため非リトライ。
func IsRetryable(err error) bool {
	return isRetryable(err)
}

// isRetryable は IsRetryable の内部実装。
func isRetryable(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Code {
	case APIErrorRateLimit, APIErrorTemporary, APIErrorNetwork:
		return true
	default:
		return false
	}
}

// extractMessage は JSON ボディからエラーメッセージを抽出する。
// パース失敗時は空文字を返す。
func extractMessage(body []byte) string {
	var eb boardAPIErrorBody
	if err := json.Unmarshal(body, &eb); err != nil {
		return ""
	}
	if eb.Message != "" {
		return eb.Message
	}
	return eb.Error
}
