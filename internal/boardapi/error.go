// Package boardapi provides an HTTP client foundation for the BOARD API.
package boardapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// APIErrorCode is a string constant representing the error type.
type APIErrorCode string

const (
	// APIErrorUnauthorized represents 401 Unauthorized.
	APIErrorUnauthorized APIErrorCode = "UNAUTHORIZED"
	// APIErrorForbidden represents 403 Forbidden.
	APIErrorForbidden APIErrorCode = "FORBIDDEN"
	// APIErrorNotFound represents 404 Not Found.
	APIErrorNotFound APIErrorCode = "NOT_FOUND"
	// APIErrorRateLimit represents 429 Too Many Requests.
	APIErrorRateLimit APIErrorCode = "RATE_LIMIT"
	// APIErrorValidation represents 400/422 validation errors.
	APIErrorValidation APIErrorCode = "VALIDATION"
	// APIErrorTemporary represents 5xx temporary server errors.
	APIErrorTemporary APIErrorCode = "TEMPORARY"
	// APIErrorNetwork represents transport-level network errors.
	APIErrorNetwork APIErrorCode = "NETWORK"
	// APIErrorUnknown represents other unclassified errors.
	APIErrorUnknown APIErrorCode = "UNKNOWN"
)

// APIError represents a BOARD API error.
// It implements the error interface.
type APIError struct {
	Code       APIErrorCode
	StatusCode int
	Message    string
	Body       string        // raw response body (for debugging)
	RetryAfter time.Duration // Retry-After header value (0 means not specified)
}

// Error implements the error interface.
// Does not include secrets such as APIKey/APIToken.
func (e *APIError) Error() string {
	return fmt.Sprintf("boardapi error [%s] status=%d: %s", e.Code, e.StatusCode, e.Message)
}

// boardAPIErrorBody is the JSON structure of BOARD API error responses.
type boardAPIErrorBody struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// parseError creates a *APIError from an HTTP status code and body.
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

// ClassifyStatusCode maps an HTTP status code to an APIErrorCode.
// Exported so it can be referenced from tests.
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

// parseErrorWithHeader creates a *APIError from an HTTP response (with Retry-After support).
func parseErrorWithHeader(resp *http.Response, body []byte) *APIError {
	ae := parseError(resp.StatusCode, body)
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			ae.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return ae
}

// IsRetryable returns whether the error is retryable.
// 429, 5xx (TEMPORARY), and network errors (NETWORK) are retryable.
// 4xx (UNAUTHORIZED/FORBIDDEN/NOT_FOUND/VALIDATION) are permanent errors and not retryable.
func IsRetryable(err error) bool {
	return isRetryable(err)
}

// IsNotFound returns whether the error represents a 404 Not Found response.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == APIErrorNotFound
	}
	return false
}

// isRetryable is the internal implementation of IsRetryable.
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

// extractMessage extracts an error message from a JSON body.
// Returns an empty string on parse failure.
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
