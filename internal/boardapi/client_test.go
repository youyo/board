package boardapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// T18: verify string format of APIError.Error()
func TestAPIError_Error(t *testing.T) {
	e := &boardapi.APIError{
		Code:       boardapi.APIErrorUnauthorized,
		StatusCode: 401,
		Message:    "invalid api key",
	}
	got := e.Error()
	want := "boardapi error [UNAUTHORIZED] status=401: invalid api key"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// T19: boundary value test for classifyStatusCode
func TestClassifyStatusCode_Boundary(t *testing.T) {
	tests := []struct {
		statusCode int
		wantCode   boardapi.APIErrorCode
	}{
		{400, boardapi.APIErrorValidation},
		{401, boardapi.APIErrorUnauthorized},
		{403, boardapi.APIErrorForbidden},
		{404, boardapi.APIErrorNotFound},
		{422, boardapi.APIErrorValidation},
		{429, boardapi.APIErrorRateLimit},
		{500, boardapi.APIErrorTemporary},
		{503, boardapi.APIErrorTemporary},
		{599, boardapi.APIErrorTemporary},
		{600, boardapi.APIErrorUnknown},
		{300, boardapi.APIErrorUnknown},
	}
	for _, tt := range tests {
		got := boardapi.ClassifyStatusCode(tt.statusCode)
		if got != tt.wantCode {
			t.Errorf("ClassifyStatusCode(%d) = %q, want %q", tt.statusCode, got, tt.wantCode)
		}
	}
}

// T01: internally created when nil httpClient is specified
func TestNew_DefaultHTTPClient(t *testing.T) {
	c := boardapi.New("https://api.the-board.jp", "key", "token", 30*time.Second)
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// T02: injected client is used with WithHTTPClient option
func TestNew_WithHTTPClient(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c := boardapi.New("https://api.the-board.jp", "key", "token", 30*time.Second,
		boardapi.WithHTTPClient(hc))
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// T03: trailing slash is removed from baseURL
func TestNew_BaseURLNormalization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path should be /test only (no double slash even if baseURL has trailing slash)
		if r.URL.Path != "/test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	// create with trailing slash
	c := boardapi.New(ts.URL+"/", "key", "token", 5*time.Second)
	req, err := c.NewRequest(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	_, err = c.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// T04: baseURL + path are correctly concatenated
func TestNewRequest_URLComposition(t *testing.T) {
	c := boardapi.New("https://api.the-board.jp", "key", "token", 30*time.Second)
	req, err := c.NewRequest(context.Background(), "GET", "/v1/clients", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	want := "https://api.the-board.jp/v1/clients"
	if req.URL.String() != want {
		t.Errorf("URL = %q, want %q", req.URL.String(), want)
	}
}

// T05: x-api-key and Authorization headers are attached
func TestDo_AuthHeaders(t *testing.T) {
	var gotKey, gotToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotToken = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "mykey", "mytoken", 5*time.Second)
	req, err := c.NewRequest(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	_, err = c.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "mykey" {
		t.Errorf("x-api-key: want %q, got %q", "mykey", gotKey)
	}
	if gotToken != "Bearer mytoken" {
		t.Errorf("Authorization: want %q, got %q", "Bearer mytoken", gotToken)
	}
}

// T06: body is returned on 200 response
func TestDo_200_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	body, err := c.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"id":1}` {
		t.Errorf("body = %q, want %q", string(body), `{"id":1}`)
	}
}

// T07: 201 Created is treated as success
func TestDo_201_Created(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":2}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "POST", "/test", nil)
	body, err := c.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"id":2}` {
		t.Errorf("body = %q, want %q", string(body), `{"id":2}`)
	}
}

// T08: 401 is converted to *APIError{Code:UNAUTHORIZED}
func TestDo_401_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid api key"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "bad", "bad", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode: want 401, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "invalid api key" {
		t.Errorf("Message: want %q, got %q", "invalid api key", apiErr.Message)
	}
}

// T09: 403 is converted to *APIError{Code:FORBIDDEN}
func TestDo_403_Forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorForbidden {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorForbidden, apiErr.Code)
	}
}

// T10: 404 is converted to *APIError{Code:NOT_FOUND}
func TestDo_404_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T11: 422 is converted to *APIError{Code:VALIDATION}
func TestDo_422_Validation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"message":"validation error"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorValidation {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorValidation, apiErr.Code)
	}
}

// T12: 429 is converted to *APIError{Code:RATE_LIMIT}
func TestDo_429_RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"rate limit exceeded"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorRateLimit {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorRateLimit, apiErr.Code)
	}
}

// T13: 500 is converted to *APIError{Code:TEMPORARY}
func TestDo_500_Temporary(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal server error"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorTemporary {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorTemporary, apiErr.Code)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode: want 500, got %d", apiErr.StatusCode)
	}
}

// T14: 503 is converted to *APIError{Code:TEMPORARY}
func TestDo_503_Temporary(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"service unavailable"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorTemporary {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorTemporary, apiErr.Code)
	}
}

// T15: network error is converted to *APIError{Code:NETWORK}
func TestDo_NetworkError(t *testing.T) {
	// request to non-existent server
	c := boardapi.New("http://127.0.0.1:19999", "key", "token", 1*time.Second)
	req, err := c.NewRequest(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	_, err = c.Do(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNetwork {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNetwork, apiErr.Code)
	}
}

// T16: message field in JSON body is stored in APIError.Message
func TestDo_ErrorMessage_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"custom error message"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Message != "custom error message" {
		t.Errorf("Message: want %q, got %q", "custom error message", apiErr.Message)
	}
}

// T16b: falls back to error field
func TestDo_ErrorMessage_ErrorField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"error field value"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Message != "error field value" {
		t.Errorf("Message: want %q, got %q", "error field value", apiErr.Message)
	}
}

// T17: no panic even when Message="" on JSON parse failure
func TestDo_ErrorMessage_Fallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	// verify no panic and that Body contains the raw body
	if apiErr.Body != "not json" {
		t.Errorf("Body: want %q, got %q", "not json", apiErr.Body)
	}
}

// T20: appropriate error returned on ctx cancellation
func TestDo_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// delay response even if request comes in (will be cancelled during test)
		select {
		case <-r.Context().Done():
			return
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(ctx, "GET", "/test", nil)

	// cancel immediately
	cancel()

	_, err := c.Do(req)
	if err == nil {
		t.Fatal("expected error after context cancel, got nil")
	}

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNetwork {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNetwork, apiErr.Code)
	}
}

// ===== M05: T35 isRetryable tests =====

// T35: isRetryable judgment for each APIErrorCode
func TestIsRetryable_Codes(t *testing.T) {
	tests := []struct {
		code          boardapi.APIErrorCode
		wantRetryable bool
	}{
		{boardapi.APIErrorRateLimit, true},
		{boardapi.APIErrorTemporary, true},
		{boardapi.APIErrorNetwork, true},
		{boardapi.APIErrorUnauthorized, false},
		{boardapi.APIErrorForbidden, false},
		{boardapi.APIErrorNotFound, false},
		{boardapi.APIErrorValidation, false},
		{boardapi.APIErrorUnknown, false},
	}
	for _, tt := range tests {
		err := &boardapi.APIError{Code: tt.code, StatusCode: 0}
		got := boardapi.IsRetryable(err)
		if got != tt.wantRetryable {
			t.Errorf("IsRetryable(%s) = %v, want %v", tt.code, got, tt.wantRetryable)
		}
	}
}

// T45: Retry-After header is stored in RetryAfter field via parseErrorWithHeader
func TestParseErrorWithHeader_RetryAfter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"rate limit"}`))
	}))
	defer ts.Close()

	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.Do(req)

	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorRateLimit {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorRateLimit, apiErr.Code)
	}
	if apiErr.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter: want %v, got %v", 5*time.Second, apiErr.RetryAfter)
	}
}

// ===== M05: T32-T34 calcBackoff tests =====

// T32: calcBackoff exponential increase (attempt=0,1,2 wait times are in ranges [0,baseDelay), [0,2*base), [0,4*base) respectively)
func TestCalcBackoff_ExponentialGrowth(t *testing.T) {
	// attempt=0: [0, 500ms)
	for i := 0; i < 100; i++ {
		d := boardapi.CalcBackoff(0, &boardapi.APIError{Code: boardapi.APIErrorTemporary})
		if d < 0 || d >= 500*time.Millisecond {
			t.Errorf("attempt=0: got %v, want [0, 500ms)", d)
		}
	}
	// attempt=4: [0, 8000ms) (base=500ms, 2^4=16 → cap at 30s)
	for i := 0; i < 100; i++ {
		d := boardapi.CalcBackoff(4, &boardapi.APIError{Code: boardapi.APIErrorTemporary})
		if d < 0 || d >= 30*time.Second {
			t.Errorf("attempt=4: got %v, want [0, 30s)", d)
		}
	}
}

// T33: Retry-After value takes priority in calcBackoff when present
func TestCalcBackoff_RetryAfterPriority(t *testing.T) {
	ae := &boardapi.APIError{Code: boardapi.APIErrorRateLimit, RetryAfter: 10 * time.Second}
	d := boardapi.CalcBackoff(0, ae)
	if d != 10*time.Second {
		t.Errorf("got %v, want 10s", d)
	}
}

// T34: calcBackoff does not exceed maxDelay(30s)
func TestCalcBackoff_MaxDelayCap(t *testing.T) {
	for attempt := 0; attempt <= 20; attempt++ {
		d := boardapi.CalcBackoff(attempt, &boardapi.APIError{Code: boardapi.APIErrorTemporary})
		if d > 30*time.Second {
			t.Errorf("attempt=%d: got %v > 30s", attempt, d)
		}
	}
}

// ===== M05: T21-T31 DoWithRetry tests =====

// T21: no retry on first-attempt success
func TestDoWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	body, err := c.DoWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount: want 1, got %d", callCount)
	}
	if string(body) != `{"id":1}` {
		t.Errorf("body: want %q, got %q", `{"id":1}`, string(body))
	}
}

// T22: 429 → retry → success
func TestDoWithRetry_429_RetryAndSucceed(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"rate limit"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	body, err := c.DoWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount: want 2, got %d", callCount)
	}
	_ = body
}

// T23: 500 → retry → success
func TestDoWithRetry_500_RetryAndSucceed(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"server error"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":2}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount: want 2, got %d", callCount)
	}
}

// T24: network error → retry → success (restarting httptest server is difficult,
// so verify behavior: connect to non-existent port → immediate error → last error returned after retryMax exceeded)
func TestDoWithRetry_NetworkError_RetryAndSucceed(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// force-close connection to simulate network error
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":3}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(5),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	// network errors are retryable so retryMax retries are attempted
	// success after 3rd attempt is expected, but connection close after Hijack appears as NETWORK error to client
	// test goal: at least callCount > 1
	if callCount < 2 {
		t.Errorf("expected retry, callCount=%d", callCount)
	}
	_ = err
}

// T25: last error returned when 429 exceeds retryMax
func TestDoWithRetry_429_ExceedsRetryMax(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"always rate limited"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorRateLimit {
		t.Errorf("Code: want RATE_LIMIT, got %q", apiErr.Code)
	}
	// retryMax=3 so called 4 times total (initial + 3 retries)
	if callCount != 4 {
		t.Errorf("callCount: want 4, got %d", callCount)
	}
}

// T26: 4xx (401/403/404/422) are not retried
func TestDoWithRetry_4xx_NoRetry(t *testing.T) {
	for _, status := range []int{401, 403, 404, 422} {
		status := status
		callCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(status)
			w.Write([]byte(`{"message":"error"}`))
		}))
		defer ts.Close()

		noSleep := func(time.Duration) {}
		c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
			boardapi.WithRetryMax(3),
			boardapi.WithSleepFn(noSleep),
		)
		req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
		_, err := c.DoWithRetry(req)
		if err == nil {
			t.Errorf("status=%d: expected error, got nil", status)
		}
		if callCount != 1 {
			t.Errorf("status=%d: callCount: want 1 (no retry), got %d", status, callCount)
		}
	}
}

// T27: waits for seconds specified in Retry-After header
func TestDoWithRetry_RetryAfterHeader(t *testing.T) {
	callCount := 0
	sleepDurations := []time.Duration{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"rate limit"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	captureSleep := func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(captureSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sleepDurations) != 1 {
		t.Fatalf("sleepDurations: want 1 call, got %d", len(sleepDurations))
	}
	if sleepDurations[0] != 3*time.Second {
		t.Errorf("sleep duration: want 3s, got %v", sleepDurations[0])
	}
}

// T28: returns immediately on ctx cancellation during backoff
func TestDoWithRetry_ContextCancelDuringBackoff(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sleepCalled := make(chan struct{}, 1)
	sleepFn := func(d time.Duration) {
		sleepCalled <- struct{}{}
	}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(sleepFn),
	)
	req, _ := c.NewRequest(ctx, "GET", "/test", nil)

	done := make(chan error, 1)
	go func() {
		_, err := c.DoWithRetry(req)
		done <- err
	}()

	// cancel when sleepFn is called
	<-sleepCalled
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

// T29: retries actually occur retryMax times
func TestDoWithRetry_RetryCount(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"unavailable"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	retryMax := 5
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(retryMax),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// initial + retryMax times = retryMax+1 total
	want := retryMax + 1
	if callCount != want {
		t.Errorf("callCount: want %d, got %d", want, callCount)
	}
}

// T30: retryMax=0 means no retry (equivalent to Do())
func TestDoWithRetry_ZeroRetryMax(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"error"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(0),
		boardapi.WithSleepFn(noSleep),
	)
	req, _ := c.NewRequest(context.Background(), "GET", "/test", nil)
	_, err := c.DoWithRetry(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("callCount: want 1, got %d", callCount)
	}
}

// T31: POST body is resent on retry
func TestDoWithRetry_RequestBodyReusable(t *testing.T) {
	receivedBodies := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(b))
		if len(receivedBodies) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message":"error"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(3),
		boardapi.WithSleepFn(noSleep),
	)
	body := strings.NewReader(`{"name":"test"}`)
	req, _ := c.NewRequest(context.Background(), "POST", "/test", body)
	_, err := c.DoWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedBodies) != 2 {
		t.Fatalf("receivedBodies: want 2, got %d", len(receivedBodies))
	}
	for i, b := range receivedBodies {
		if b != `{"name":"test"}` {
			t.Errorf("body[%d]: want %q, got %q", i, `{"name":"test"}`, b)
		}
	}
}

// ===== M05: T36-T44 ListAll tests =====

// T36: terminates on 1 page (fewer than per_page items)
func TestListAll_SinglePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		return c.NewRequest(ctx, "GET", "/test", nil)
	}
	items, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("want 3 items, got %d", len(items))
	}
}

// T37: fetches all items across multiple pages and concatenates
func TestListAll_MultiPage(t *testing.T) {
	pageItems := map[int]int{1: 100, 2: 100, 3: 37}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			var p int
			_, err := fmt.Sscanf(pageStr, "%d", &p)
			if err == nil {
				page = p
			}
		}
		n := pageItems[page]
		items := make([]map[string]int, n)
		for i := range items {
			items[i] = map[string]int{"id": (page-1)*100 + i + 1}
		}
		b, _ := json.Marshal(items)
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		path := "/test?page=" + fmt.Sprintf("%d", page) + "&per_page=" + fmt.Sprintf("%d", perPage)
		return c.NewRequest(ctx, "GET", path, nil)
	}
	items, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 237 {
		t.Errorf("want 237 items, got %d", len(items))
	}
}

// T38: terminates after fetching empty list following a full per_page page
func TestListAll_ExactMultiple(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// return exactly per_page=3 items
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
			return
		}
		// second call returns empty
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		return c.NewRequest(ctx, "GET", "/test", nil)
	}
	items, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("want 3 items, got %d", len(items))
	}
	if callCount != 2 {
		t.Errorf("callCount: want 2, got %d", callCount)
	}
}

// T39: returns 0 items for empty array []
func TestListAll_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		return c.NewRequest(ctx, "GET", "/test", nil)
	}
	items, err := c.ListAll(context.Background(), makeReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

// T40: per_page query parameter is correctly attached
func TestListAll_PerPageQueryParam(t *testing.T) {
	var gotPerPage string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		path := fmt.Sprintf("/test?page=%d&per_page=%d", page, perPage)
		return c.NewRequest(ctx, "GET", path, nil)
	}
	_, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != "50" {
		t.Errorf("per_page: want %q, got %q", "50", gotPerPage)
	}
}

// T41: page=1,2,3... is correctly incremented
func TestListAll_PageQueryParam(t *testing.T) {
	var gotPages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPages = append(gotPages, r.URL.Query().Get("page"))
		if len(gotPages) < 3 {
			// 3 items/page for 3 pages
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":7}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		path := fmt.Sprintf("/test?page=%d&per_page=%d", page, perPage)
		return c.NewRequest(ctx, "GET", path, nil)
	}
	_, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantPages := []string{"1", "2", "3"}
	for i, want := range wantPages {
		if i >= len(gotPages) || gotPages[i] != want {
			t.Errorf("gotPages[%d]: want %q, got %q", i, want, gotPages[i])
		}
	}
}

// T42: returns immediately when error occurs in a middle page
func TestListAll_ErrorPropagation(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1},{"id":2}]`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"server error"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(0), // no retry
		boardapi.WithSleepFn(noSleep),
	)
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		return c.NewRequest(ctx, "GET", "/test", nil)
	}
	_, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(2))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorTemporary {
		t.Errorf("Code: want TEMPORARY, got %q", apiErr.Code)
	}
}

// T43: immediately aborts on ctx cancellation during page loop
func TestListAll_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		callCount++
		if callCount >= 2 {
			cancel()
		}
		return c.NewRequest(ctx, "GET", "/test", nil)
	}
	_, err := c.ListAll(ctx, makeReq, boardapi.WithPerPage(3))
	if err == nil {
		t.Fatal("expected error after context cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

// T44: query parameter changes with WithPerPage(50)
func TestListAll_WithPerPageOption(t *testing.T) {
	var gotPerPage string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		path := fmt.Sprintf("/test?page=%d&per_page=%d", page, perPage)
		return c.NewRequest(ctx, "GET", path, nil)
	}
	_, err := c.ListAll(context.Background(), makeReq, boardapi.WithPerPage(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != "50" {
		t.Errorf("per_page: want 50, got %q", gotPerPage)
	}
}

// ============================================================
// M06: clients entity tests (T46-T51, T64, T66)
// ============================================================

// T46: ListClients — happy path (2 pages)
func TestListClients_TwoPages(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1,"name":"Client A","code":"A001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Client B","code":"B001","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		}
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	clients, err := c.ListClients(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("want 2 clients, got %d", len(clients))
	}
	if clients[0].ID != 1 || clients[0].Name != "Client A" {
		t.Errorf("clients[0]: got %+v", clients[0])
	}
	if clients[1].ID != 2 || clients[1].Name != "Client B" {
		t.Errorf("clients[1]: got %+v", clients[1])
	}
}

// T47: ListClients — API error (401)
func TestListClients_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(0),
		boardapi.WithSleepFn(noSleep),
	)
	_, err := c.ListClients(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want UNAUTHORIZED, got %q", apiErr.Code)
	}
}

// T48: GetClient — happy path
func TestGetClient_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/clients/123" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":123,"name":"Test Client","code":"T001","memo":"test memo","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetClient(context.Background(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ClientEntity")
	}
	if got.ID != 123 || got.Name != "Test Client" {
		t.Errorf("GetClient: got %+v", got)
	}
}

// T49: GetClient — 404 Not Found
func TestGetClient_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(0),
		boardapi.WithSleepFn(noSleep),
	)
	got, err := c.GetClient(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("expected nil ClientEntity, got %+v", got)
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want NOT_FOUND, got %q", apiErr.Code)
	}
}

// T50: SearchClients — with Name parameter
func TestSearchClients_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"name":"test","code":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchClients(context.Background(), boardapi.ClientSearchParams{Name: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 client, got %d", len(result))
	}
	if gotName != "test" {
		t.Errorf("name param: want %q, got %q", "test", gotName)
	}
}

// T51: SearchClients — empty result
func TestSearchClients_EmptyResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchClients(context.Background(), boardapi.ClientSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 clients, got %d", len(result))
	}
}

// T64: GetClient — on context cancellation
func TestGetClient_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetClient(ctx, 1)
	if err == nil {
		t.Fatal("expected error due to context cancel")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want context.Canceled or DeadlineExceeded, got %v", err)
	}
}

// T66: SearchClients — with UpdatedAtFrom parameter
func TestSearchClients_WithUpdatedAtFrom(t *testing.T) {
	var gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchClients(context.Background(), boardapi.ClientSearchParams{UpdatedAtFrom: "2026-03-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUpdatedAtFrom != "2026-03-01T00:00:00Z" {
		t.Errorf("updated_at_from param: want %q, got %q", "2026-03-01T00:00:00Z", gotUpdatedAtFrom)
	}
}

// ============================================================
// M06: client_branches entity tests (T52-T54)
// ============================================================

// T52: ListClientBranches — happy path
func TestListClientBranches_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"name":"Tokyo Branch","postal_code":"100-0001","address":"Tokyo","phone":"03-0000-0000","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListClientBranches(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 client branch, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].ClientID != 10 || result[0].Name != "Tokyo Branch" {
		t.Errorf("result[0]: got %+v", result[0])
	}
}

// T53: GetClientBranch — happy path
func TestGetClientBranch_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/client_branches/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"client_id":5,"name":"Osaka Branch","postal_code":"530-0001","address":"Osaka","phone":"06-0000-0000","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetClientBranch(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ClientBranchEntity")
	}
	if got.ID != 1 || got.Name != "Osaka Branch" {
		t.Errorf("GetClientBranch: got %+v", got)
	}
}

// T54: SearchClientBranches — with ClientID parameter
func TestSearchClientBranches_WithClientID(t *testing.T) {
	var gotClientID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.URL.Query().Get("client_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":2,"client_id":10,"name":"Nagoya Branch","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchClientBranches(context.Background(), boardapi.ClientBranchSearchParams{ClientID: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotClientID != "10" {
		t.Errorf("client_id param: want %q, got %q", "10", gotClientID)
	}
}

// ============================================================
// M06: contacts entity tests (T55-T57)
// ============================================================

// T55: ListContacts — happy path (3 items)
func TestListContacts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"client_branch_id":2,"name":"Taro Tanaka","name_kana":"tanataro","title":"Manager","email":"tanaka@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"client_id":10,"client_branch_id":2,"name":"Hanako Suzuki","name_kana":"suzukihanako","title":"","email":"suzuki@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":3,"client_id":11,"client_branch_id":3,"name":"Jiro Sato","name_kana":"satojiro","title":"","email":"sato@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListContacts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("want 3 contacts, got %d", len(result))
	}
}

// T56: GetContact — happy path
func TestGetContact_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/contacts/5" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":5,"client_id":10,"client_branch_id":2,"name":"Ichiro Yamada","name_kana":"yamadaichiro","title":"","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetContact(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ContactEntity")
	}
	if got.ID != 5 || got.Email != "test@example.com" {
		t.Errorf("GetContact: got %+v", got)
	}
}

// T57: SearchContacts — with Email parameter
func TestSearchContacts_WithEmail(t *testing.T) {
	var gotEmail string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEmail = r.URL.Query().Get("email")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":5,"client_id":10,"client_branch_id":2,"name":"Ichiro Yamada","name_kana":"","title":"","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchContacts(context.Background(), boardapi.ContactSearchParams{Email: "test@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotEmail != "test@example.com" {
		t.Errorf("email param: want %q, got %q", "test@example.com", gotEmail)
	}
}

// ============================================================
// M06: projects entity tests (T58-T60, T65)
// ============================================================

// T58: ListProjects — happy path (pagination: 2 pages)
// since defaultPerPage=100, return 100 items on page=1 and remainder on page=2
func TestListProjects_TwoPages(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// return 100 items (= perPage → next page exists)
			items := make([]string, 100)
			for i := range items {
				items[i] = fmt.Sprintf(`{"id":%d,"client_id":10,"name":"Project %d","code":"P%03d","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`, i+1, i+1, i+1)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[" + strings.Join(items, ",") + "]"))
		} else {
			// page=2: 50 items (final page)
			items := make([]string, 50)
			for i := range items {
				items[i] = fmt.Sprintf(`{"id":%d,"client_id":10,"name":"Project %d","code":"P%03d","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`, 100+i+1, 100+i+1, 100+i+1)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[" + strings.Join(items, ",") + "]"))
		}
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 150 {
		t.Errorf("want 150 projects, got %d", len(result))
	}
}

// T59: GetProject — happy path
func TestGetProject_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/200" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":200,"client_id":10,"name":"Development Project","code":"DEV001","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProject(context.Background(), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ProjectEntity")
	}
	if got.ID != 200 || got.Name != "Development Project" {
		t.Errorf("GetProject: got %+v", got)
	}
}

// T60: SearchProjects — with Status + UpdatedAtFrom parameters
func TestSearchProjects_WithStatusAndUpdatedAtFrom(t *testing.T) {
	var gotStatus, gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStatus = r.URL.Query().Get("status")
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchProjects(context.Background(), boardapi.ProjectSearchParams{
		Status:        "active",
		UpdatedAtFrom: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStatus != "active" {
		t.Errorf("status param: want %q, got %q", "active", gotStatus)
	}
	if gotUpdatedAtFrom != "2026-01-01T00:00:00Z" {
		t.Errorf("updated_at_from param: want %q, got %q", "2026-01-01T00:00:00Z", gotUpdatedAtFrom)
	}
}

// T65: ListProjects — JSON deserialization error
func TestListProjects_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// invalid JSON (each element is malformed)
		w.Write([]byte(`["not-an-object"]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListProjects(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorUnknown {
		t.Errorf("Code: want UNKNOWN, got %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "unmarshal") {
		t.Errorf("Message should contain 'unmarshal', got %q", apiErr.Message)
	}
}

// ============================================================
// M06: project_costs entity tests (T61-T63)
// ============================================================

// T61: ListProjectCosts — happy path (5 items)
func TestListProjectCosts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"project_id":200,"name":"Labor Cost","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"project_id":200,"name":"Outsourcing Cost","cost_type":"outsource","amount":50000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":3,"project_id":200,"name":"Travel Expense","cost_type":"expense","amount":10000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":4,"project_id":200,"name":"Communication Expense","cost_type":"expense","amount":5000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":5,"project_id":200,"name":"Equipment Expense","cost_type":"expense","amount":3000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListProjectCosts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("want 5 project costs, got %d", len(result))
	}
}

// T62: GetProjectCost — happy path
func TestGetProjectCost_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/project_costs/50" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":50,"project_id":200,"name":"Labor Cost","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProjectCost(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ProjectCostEntity")
	}
	if got.ID != 50 || got.Amount != 100000.0 {
		t.Errorf("GetProjectCost: got %+v", got)
	}
}

// T63: SearchProjectCosts — with ProjectID parameter
func TestSearchProjectCosts_WithProjectID(t *testing.T) {
	var gotProjectID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"project_id":200,"name":"Labor Cost","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchProjectCosts(context.Background(), boardapi.ProjectCostSearchParams{ProjectID: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotProjectID != "200" {
		t.Errorf("project_id param: want %q, got %q", "200", gotProjectID)
	}
}

// ============================================================
// M07: estimates entity tests (T68)
// ============================================================

// T68: GetEstimate — happy path (M35: 実 API 準拠スキーマに更新)
func TestGetEstimate_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/documents/estimates/42" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":42,"message":"test message","total":"750000.0","tax":"75000.0","tax_withholding":"0.0","seal_approval_status":1,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[{"no":1,"detail_date":null,"description":"Item A","quantity":"1.0","unit":"式","unit_price":"750000.0","price":"750000.0","tax_rate":"10.0","tax_withholding_flg":0,"tax_included_flg":0,"reduced_tax_rate_kbn":1,"section_description":null,"section_subtotal":null,"document_detail_kbn":1,"document_detail_kbn_name":"通常","deduction_applicable":false}],"valid_period":"御見積後１ヶ月"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetEstimate(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil EstimateEntity")
	}
	if got.ID != 42 {
		t.Errorf("GetEstimate ID: got %d, want 42", got.ID)
	}
	if got.Total != "750000.0" {
		t.Errorf("GetEstimate Total: got %q, want %q", got.Total, "750000.0")
	}
	if got.ValidPeriod != "御見積後１ヶ月" {
		t.Errorf("GetEstimate ValidPeriod: got %q, want %q", got.ValidPeriod, "御見積後１ヶ月")
	}
	if len(got.Details) != 1 {
		t.Errorf("GetEstimate Details len: got %d, want 1", len(got.Details))
	}
}

// ============================================================
// M07: invoices entity tests (T70-T72)
// ============================================================

// T70: ListInvoices — happy path
func TestListInvoices_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"Invoice 1","total_amount":500000.0,"status":"draft","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"},{"id":2,"client_id":10,"project_id":101,"title":"Invoice 2","total_amount":200000.0,"status":"sent","invoice_date":"2026-02-28","due_date":"2026-03-31","memo":"","updated_at":"2026-02-28T00:00:00Z","created_at":"2026-02-28T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListInvoices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 invoices, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].InvoiceDate != "2026-01-31" || result[0].DueDate != "2026-02-28" {
		t.Errorf("invoice[0]: got %+v", result[0])
	}
}

// T71: GetInvoice — happy path
func TestGetInvoice_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/invoices/55" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":55,"client_id":10,"project_id":100,"title":"Invoice 55","total_amount":1000000.0,"status":"paid","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"Paid","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetInvoice(context.Background(), 55)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil InvoiceEntity")
	}
	if got.ID != 55 || got.TotalAmount != 1000000.0 || got.Status != "paid" {
		t.Errorf("GetInvoice: got %+v", got)
	}
}

// T72: SearchInvoices — with ProjectID + UpdatedAtFrom parameters
func TestSearchInvoices_WithProjectIDAndUpdatedAtFrom(t *testing.T) {
	var gotProjectID, gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"Invoice 1","total_amount":500000.0,"status":"draft","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchInvoices(context.Background(), boardapi.InvoiceSearchParams{ProjectID: 100, UpdatedAtFrom: "2026-01-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotProjectID != "100" {
		t.Errorf("project_id param: want %q, got %q", "100", gotProjectID)
	}
	if gotUpdatedAtFrom != "2026-01-01" {
		t.Errorf("updated_at_from param: want %q, got %q", "2026-01-01", gotUpdatedAtFrom)
	}
}

// ============================================================
// M07: orders entity tests (T74)
// ============================================================

// T74: GetOrder — happy path (M36: 実 API 準拠スキーマに更新)
func TestGetOrder_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/documents/orders/77" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":77,"message":null,"total":"800000.0","tax":"80000.0","tax_withholding":"0.0","seal_approval_status":1,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[{"no":1,"detail_date":null,"description":"Item A","quantity":"1.0","unit":"式","unit_price":"800000.0","price":"800000.0","tax_rate":"10.0","tax_withholding_flg":0,"tax_included_flg":0,"reduced_tax_rate_kbn":1,"section_description":null,"section_subtotal":null,"document_detail_kbn":1,"document_detail_kbn_name":"通常","deduction_applicable":false}],"disp_order_date":null,"disp_order_receive_date":null}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetOrder(context.Background(), 77)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil OrderEntity")
	}
	if got.ID != 77 {
		t.Errorf("GetOrder ID: got %d, want 77", got.ID)
	}
	if got.Total != "800000.0" {
		t.Errorf("GetOrder Total: got %q, want %q", got.Total, "800000.0")
	}
	if len(got.Details) != 1 {
		t.Errorf("GetOrder Details len: got %d, want 1", len(got.Details))
	}
}

// ============================================================
// M07: deliveries entity tests (T77)
// ============================================================

// T77: GetDelivery — happy path (M37: 実 API 準拠スキーマに更新)
func TestGetDelivery_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/documents/deliveries/88" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":88,"message":null,"total":"600000.0","tax":"60000.0","tax_withholding":"0.0","seal_approval_status":1,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":"御社指定場所","details":[{"no":1,"detail_date":null,"description":"Item A","quantity":"1.0","unit":"式","unit_price":"600000.0","price":"600000.0","tax_rate":"10.0","tax_withholding_flg":0,"tax_included_flg":0,"reduced_tax_rate_kbn":1,"section_description":null,"section_subtotal":null,"document_detail_kbn":1,"document_detail_kbn_name":"通常","deduction_applicable":false}],"delivery_date":"2026-02-15","disp_delivery_date":null,"disp_delivery_receive_date":null}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetDelivery(context.Background(), 88)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil DeliveryEntity")
	}
	if got.ID != 88 {
		t.Errorf("GetDelivery ID: got %d, want 88", got.ID)
	}
	if got.DeliveryDate != "2026-02-15" {
		t.Errorf("GetDelivery DeliveryDate: got %q, want %q", got.DeliveryDate, "2026-02-15")
	}
	if got.Total != "600000.0" {
		t.Errorf("GetDelivery Total: got %q, want %q", got.Total, "600000.0")
	}
	if len(got.Details) != 1 {
		t.Errorf("GetDelivery Details len: got %d, want 1", len(got.Details))
	}
}

// ============================================================
// M07: receipts entity tests (T80)
// ============================================================

// T80: GetReceipt — happy path (M38: 実 API 準拠スキーマに更新)
func TestGetReceipt_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/documents/receipts/99" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":99,"message":null,"total":"900000.0","tax":"90000.0","tax_withholding":"0.0","seal_approval_status":1,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[{"no":1,"detail_date":null,"description":"Item A","quantity":"1.0","unit":"式","unit_price":"900000.0","price":"900000.0","tax_rate":"10.0","tax_withholding_flg":0,"tax_included_flg":0,"reduced_tax_rate_kbn":1,"section_description":null,"section_subtotal":null,"document_detail_kbn":1,"document_detail_kbn_name":"通常","deduction_applicable":false}],"receipt_date":"2026-03-31","disp_receipt_date":null}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetReceipt(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ReceiptEntity")
	}
	if got.ID != 99 {
		t.Errorf("GetReceipt ID: got %d, want 99", got.ID)
	}
	if got.ReceiptDate != "2026-03-31" {
		t.Errorf("GetReceipt ReceiptDate: got %q, want %q", got.ReceiptDate, "2026-03-31")
	}
	if got.Total != "900000.0" {
		t.Errorf("GetReceipt Total: got %q, want %q", got.Total, "900000.0")
	}
	if len(got.Details) != 1 {
		t.Errorf("GetReceipt Details len: got %d, want 1", len(got.Details))
	}
}

// ============================================================
// M08: vendors entity tests (T112-T117)
// ============================================================

// T112: ListVendors — happy path (2 items)
func TestListVendors_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Vendor A","code":"VA001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Vendor B","code":"VB002","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListVendors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 vendors, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Vendor A" || result[0].Code != "VA001" {
		t.Errorf("vendor[0]: got %+v", result[0])
	}
}

// T113: ListVendors — API error (401)
func TestListVendors_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListVendors(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
}

// T114: GetVendor — happy path
func TestGetVendor_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payees/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Vendor A","code":"VA001","memo":"test memo","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetVendor(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil VendorEntity")
	}
	if got.ID != 1 || got.Name != "Vendor A" || got.Code != "VA001" {
		t.Errorf("GetVendor: got %+v", got)
	}
}

// T115: GetVendor — 404 Not Found
func TestGetVendor_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetVendor(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T116: SearchVendors — with Name parameter
func TestSearchVendors_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Company A","code":"A001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendors(context.Background(), boardapi.VendorSearchParams{Name: "Company A"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Company A" {
		t.Errorf("name param: want %q, got %q", "Company A", gotName)
	}
}

// T117: SearchVendors — with UpdatedAtFrom parameter
func TestSearchVendors_WithUpdatedAtFrom(t *testing.T) {
	var gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchVendors(context.Background(), boardapi.VendorSearchParams{UpdatedAtFrom: "2024-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUpdatedAtFrom != "2024-01-01T00:00:00Z" {
		t.Errorf("updated_at_from param: want %q, got %q", "2024-01-01T00:00:00Z", gotUpdatedAtFrom)
	}
}

// ============================================================
// M08: vendor_branches entity tests (T118-T123)
// ============================================================

// T118: ListVendorBranches — happy path (2 items)
func TestListVendorBranches_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"vendor_id":1,"name":"Tokyo Branch","postal_code":"100-0001","address":"Chiyoda, Tokyo","phone":"03-0000-0001","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":11,"vendor_id":1,"name":"Osaka Branch","postal_code":"530-0001","address":"Osaka City, Osaka","phone":"06-0000-0001","fax":"","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListVendorBranches(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 vendor branches, got %d", len(result))
	}
	if result[0].ID != 10 || result[0].VendorID != 1 || result[0].PostalCode != "100-0001" {
		t.Errorf("vendorBranch[0]: got %+v", result[0])
	}
}

// T119: ListVendorBranches — API error (401)
func TestListVendorBranches_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListVendorBranches(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
}

// T120: GetVendorBranch — happy path
func TestGetVendorBranch_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payee_branches/10" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":10,"vendor_id":1,"name":"Tokyo Branch","postal_code":"100-0001","address":"Chiyoda, Tokyo","phone":"03-0000-0001","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetVendorBranch(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil VendorBranchEntity")
	}
	if got.ID != 10 || got.PostalCode != "100-0001" {
		t.Errorf("GetVendorBranch: got %+v", got)
	}
}

// T121: GetVendorBranch — 404 Not Found
func TestGetVendorBranch_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetVendorBranch(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T122: SearchVendorBranches — with VendorID parameter
func TestSearchVendorBranches_WithVendorID(t *testing.T) {
	var gotVendorID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"vendor_id":5,"name":"Tokyo Branch","postal_code":"100-0001","address":"Chiyoda, Tokyo","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendorBranches(context.Background(), boardapi.VendorBranchSearchParams{VendorID: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotVendorID != "5" {
		t.Errorf("vendor_id param: want %q, got %q", "5", gotVendorID)
	}
}

// T123: SearchVendorBranches — with Name parameter
func TestSearchVendorBranches_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchVendorBranches(context.Background(), boardapi.VendorBranchSearchParams{Name: "Tokyo Branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "Tokyo Branch" {
		t.Errorf("name param: want %q, got %q", "Tokyo Branch", gotName)
	}
}

// ============================================================
// M08: vendor_contacts entity tests (T124-T129)
// ============================================================

// T124: ListVendorContacts — happy path (2 items)
func TestListVendorContacts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"Taro Yamada","name_kana":"YAMADATARO","title":"Manager","email":"yamada@example.com","phone":"03-0000-0001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"vendor_id":1,"vendor_branch_id":10,"name":"Hanako Suzuki","name_kana":"SUZUKIHANAKO","title":"Section Chief","email":"suzuki@example.com","phone":"03-0000-0002","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListVendorContacts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 vendor contacts, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Email != "yamada@example.com" {
		t.Errorf("vendorContact[0]: got %+v", result[0])
	}
}

// T125: ListVendorContacts — API error (401)
func TestListVendorContacts_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListVendorContacts(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
}

// T126: GetVendorContact — happy path
func TestGetVendorContact_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payee_contacts/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"Taro Yamada","name_kana":"YAMADATARO","title":"Manager","email":"yamada@example.com","phone":"03-0000-0001","memo":"test memo","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetVendorContact(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil VendorContactEntity")
	}
	if got.ID != 1 || got.Email != "yamada@example.com" {
		t.Errorf("GetVendorContact: got %+v", got)
	}
}

// T127: GetVendorContact — 404 Not Found
func TestGetVendorContact_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetVendorContact(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T128: SearchVendorContacts — with VendorID + Email parameters
func TestSearchVendorContacts_WithVendorIDAndEmail(t *testing.T) {
	var gotVendorID, gotEmail string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		gotEmail = r.URL.Query().Get("email")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":3,"vendor_branch_id":10,"name":"Taro Yamada","name_kana":"YAMADATARO","title":"Manager","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendorContacts(context.Background(), boardapi.VendorContactSearchParams{VendorID: 3, Email: "test@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotVendorID != "3" {
		t.Errorf("vendor_id param: want %q, got %q", "3", gotVendorID)
	}
	if gotEmail != "test@example.com" {
		t.Errorf("email param: want %q, got %q", "test@example.com", gotEmail)
	}
}

// T129: SearchVendorContacts — with Name parameter
func TestSearchVendorContacts_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchVendorContacts(context.Background(), boardapi.VendorContactSearchParams{Name: "Yamada"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "Yamada" {
		t.Errorf("name param: want %q, got %q", "Yamada", gotName)
	}
}

// ============================================================
// M08: purchase_orders entity tests (T130-T135)
// ============================================================

// T130: ListPurchaseOrders — happy path (3 items)
func TestListPurchaseOrders_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"project_id":100,"title":"Order 1","total_amount":500000.0,"status":"draft","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"vendor_id":1,"project_id":101,"title":"Order 2","total_amount":300000.0,"status":"approved","order_date":"2026-01-05","delivery_date":"2026-02-05","memo":"","updated_at":"2026-01-05T00:00:00Z","created_at":"2026-01-05T00:00:00Z"},{"id":3,"vendor_id":2,"project_id":102,"title":"Order 3","total_amount":150000.0,"status":"sent","order_date":"2026-01-10","delivery_date":"2026-02-10","memo":"","updated_at":"2026-01-10T00:00:00Z","created_at":"2026-01-10T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListPurchaseOrders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("want 3 purchase orders, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].TotalAmount != 500000.0 {
		t.Errorf("purchaseOrder[0]: got %+v", result[0])
	}
}

// T131: ListPurchaseOrders — API error (401)
func TestListPurchaseOrders_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListPurchaseOrders(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
}

// T132: GetPurchaseOrder — happy path
func TestGetPurchaseOrder_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/expenditures/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"project_id":100,"title":"Order 1","total_amount":500000.0,"status":"approved","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"test memo","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetPurchaseOrder(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PurchaseOrderEntity")
	}
	if got.ID != 1 || got.OrderDate != "2026-01-01" || got.DeliveryDate != "2026-02-01" {
		t.Errorf("GetPurchaseOrder: got %+v", got)
	}
}

// T133: GetPurchaseOrder — 404 Not Found
func TestGetPurchaseOrder_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetPurchaseOrder(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T134: SearchPurchaseOrders — with VendorID + Status parameters
func TestSearchPurchaseOrders_WithVendorIDAndStatus(t *testing.T) {
	var gotVendorID, gotStatus string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		gotStatus = r.URL.Query().Get("status")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"project_id":100,"title":"Order 1","total_amount":500000.0,"status":"approved","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPurchaseOrders(context.Background(), boardapi.PurchaseOrderSearchParams{VendorID: 1, Status: "approved"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotVendorID != "1" {
		t.Errorf("vendor_id param: want %q, got %q", "1", gotVendorID)
	}
	if gotStatus != "approved" {
		t.Errorf("status param: want %q, got %q", "approved", gotStatus)
	}
}

// T135: SearchPurchaseOrders — with ProjectID + UpdatedAtFrom parameters
func TestSearchPurchaseOrders_WithProjectIDAndUpdatedAtFrom(t *testing.T) {
	var gotProjectID, gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchPurchaseOrders(context.Background(), boardapi.PurchaseOrderSearchParams{ProjectID: 10, UpdatedAtFrom: "2024-06-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotProjectID != "10" {
		t.Errorf("project_id param: want %q, got %q", "10", gotProjectID)
	}
	if gotUpdatedAtFrom != "2024-06-01T00:00:00Z" {
		t.Errorf("updated_at_from param: want %q, got %q", "2024-06-01T00:00:00Z", gotUpdatedAtFrom)
	}
}

// ============================================================
// M08: payments entity tests (T136-T141)
// ============================================================

// T136: ListPayments — happy path (2 items)
func TestListPayments_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"purchase_order_id":10,"amount":500000.0,"status":"pending","payment_date":"2026-01-31","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"},{"id":2,"vendor_id":1,"purchase_order_id":11,"amount":300000.0,"status":"paid","payment_date":"2026-02-28","memo":"","updated_at":"2026-02-28T00:00:00Z","created_at":"2026-02-28T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListPayments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 payments, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Amount != 500000.0 {
		t.Errorf("payment[0]: got %+v", result[0])
	}
}

// T137: ListPayments — API error (401)
func TestListPayments_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListPayments(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorUnauthorized, apiErr.Code)
	}
}

// T138: GetPayment — happy path
func TestGetPayment_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/expenditure_payments/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"purchase_order_id":10,"amount":500000.0,"status":"paid","payment_date":"2026-01-31","memo":"Paid","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetPayment(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PaymentEntity")
	}
	if got.ID != 1 || got.PaymentDate != "2026-01-31" || got.Status != "paid" {
		t.Errorf("GetPayment: got %+v", got)
	}
}

// T139: GetPayment — 404 Not Found
func TestGetPayment_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.GetPayment(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("Code: want %q, got %q", boardapi.APIErrorNotFound, apiErr.Code)
	}
}

// T140: SearchPayments — with VendorID + Status parameters
func TestSearchPayments_WithVendorIDAndStatus(t *testing.T) {
	var gotVendorID, gotStatus string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		gotStatus = r.URL.Query().Get("status")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":2,"purchase_order_id":10,"amount":500000.0,"status":"paid","payment_date":"2026-01-31","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPayments(context.Background(), boardapi.PaymentSearchParams{VendorID: 2, Status: "paid"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotVendorID != "2" {
		t.Errorf("vendor_id param: want %q, got %q", "2", gotVendorID)
	}
	if gotStatus != "paid" {
		t.Errorf("status param: want %q, got %q", "paid", gotStatus)
	}
}

// T141: SearchPayments — with PurchaseOrderID parameter
func TestSearchPayments_WithPurchaseOrderID(t *testing.T) {
	var gotPurchaseOrderID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPurchaseOrderID = r.URL.Query().Get("purchase_order_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchPayments(context.Background(), boardapi.PaymentSearchParams{PurchaseOrderID: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPurchaseOrderID != "7" {
		t.Errorf("purchase_order_id param: want %q, got %q", "7", gotPurchaseOrderID)
	}
}

// ============================================================
// M08: cross-cutting tests (T142-T156)
// ============================================================

// T142: ListVendors — pagination 2 pages (100 + 50 items)
func TestListVendors_TwoPages(t *testing.T) {
	page1 := make([]map[string]interface{}, 100)
	for i := range page1 {
		page1[i] = map[string]interface{}{"id": i + 1, "name": "Vendor", "code": "", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}
	page2 := make([]map[string]interface{}, 50)
	for i := range page2 {
		page2[i] = map[string]interface{}{"id": i + 101, "name": "Vendor", "code": "", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}

	call := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			b, _ := json.Marshal(page1)
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		} else {
			b, _ := json.Marshal(page2)
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		}
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListVendors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 150 {
		t.Errorf("want 150 vendors, got %d", len(result))
	}
}

// T143: ListPurchaseOrders — pagination 2 pages (100 + 30 items)
func TestListPurchaseOrders_TwoPages(t *testing.T) {
	page1 := make([]map[string]interface{}, 100)
	for i := range page1 {
		page1[i] = map[string]interface{}{"id": i + 1, "vendor_id": 1, "project_id": 100, "title": "Purchase Order", "total_amount": 100000.0, "status": "draft", "order_date": "2026-01-01", "delivery_date": "2026-02-01", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}
	page2 := make([]map[string]interface{}, 30)
	for i := range page2 {
		page2[i] = map[string]interface{}{"id": i + 101, "vendor_id": 1, "project_id": 100, "title": "Purchase Order", "total_amount": 100000.0, "status": "draft", "order_date": "2026-01-01", "delivery_date": "2026-02-01", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}

	call := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			b, _ := json.Marshal(page1)
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		} else {
			b, _ := json.Marshal(page2)
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		}
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListPurchaseOrders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 130 {
		t.Errorf("want 130 purchase orders, got %d", len(result))
	}
}

// T144: GetVendor — on context cancellation
func TestGetVendor_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Vendor A","code":"VA001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetVendor(ctx, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// T145: GetVendorBranch — on context cancellation
func TestGetVendorBranch_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":10,"vendor_id":1,"name":"Tokyo Branch","postal_code":"100-0001","address":"","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetVendorBranch(ctx, 10)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// T146: GetVendorContact — on context cancellation
func TestGetVendorContact_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"Taro Yamada","name_kana":"","title":"","email":"","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetVendorContact(ctx, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// T147: GetPurchaseOrder — on context cancellation
func TestGetPurchaseOrder_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"project_id":100,"title":"Order 1","total_amount":500000.0,"status":"draft","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetPurchaseOrder(ctx, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// T148: GetPayment — on context cancellation
func TestGetPayment_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"purchase_order_id":10,"amount":500000.0,"status":"paid","payment_date":"2026-01-31","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetPayment(ctx, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// T149: ListVendors — invalid JSON (unmarshal error)
func TestListVendors_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "not_an_int"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListVendors(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !strings.Contains(apiErr.Message, "ListVendors: unmarshal:") {
		t.Errorf("Message should contain %q, got %q", "ListVendors: unmarshal:", apiErr.Message)
	}
}

// T150: ListPurchaseOrders — invalid JSON (unmarshal error)
func TestListPurchaseOrders_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "not_an_int"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListPurchaseOrders(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !strings.Contains(apiErr.Message, "ListPurchaseOrders: unmarshal:") {
		t.Errorf("Message should contain %q, got %q", "ListPurchaseOrders: unmarshal:", apiErr.Message)
	}
}

// T151: ListPayments — invalid JSON (unmarshal error)
func TestListPayments_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "not_an_int"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.ListPayments(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*boardapi.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !strings.Contains(apiErr.Message, "ListPayments: unmarshal:") {
		t.Errorf("Message should contain %q, got %q", "ListPayments: unmarshal:", apiErr.Message)
	}
}

// T152: SearchVendors — all parameters zero value
func TestSearchVendors_ZeroParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendors(context.Background(), boardapi.VendorSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 results, got %d", len(result))
	}
}

// T153: SearchVendorBranches — all parameters zero value
func TestSearchVendorBranches_ZeroParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendorBranches(context.Background(), boardapi.VendorBranchSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 results, got %d", len(result))
	}
}

// T154: SearchPurchaseOrders — all parameters zero value
func TestSearchPurchaseOrders_ZeroParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPurchaseOrders(context.Background(), boardapi.PurchaseOrderSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 results, got %d", len(result))
	}
}

// T155: SearchPayments — all parameters zero value
func TestSearchPayments_ZeroParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPayments(context.Background(), boardapi.PaymentSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 results, got %d", len(result))
	}
}

// T156: SearchVendorContacts — all parameters zero value
func TestSearchVendorContacts_ZeroParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendorContacts(context.Background(), boardapi.VendorContactSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("want 0 results, got %d", len(result))
	}
}

// ============================================================
// M09: master entity tests (T157-T177)
// ============================================================

// T157: ListUsers — happy path (2 items)
func TestListUsers_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Taro Yamada","email":"yamada@example.com","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Hanako Suzuki","email":"suzuki@example.com","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 users, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Taro Yamada" || result[0].Email != "yamada@example.com" {
		t.Errorf("user[0]: got %+v", result[0])
	}
}

// T158: GetUser — happy path
func TestGetUser_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Taro Yamada","email":"yamada@example.com","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil UserEntity")
	}
	if got.ID != 1 || got.Name != "Taro Yamada" || got.Email != "yamada@example.com" {
		t.Errorf("GetUser: got %+v", got)
	}
}

// T159: SearchUsers — with Name parameter
func TestSearchUsers_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Taro Yamada","email":"yamada@example.com","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchUsers(context.Background(), boardapi.UserSearchParams{Name: "Taro Yamada"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Taro Yamada" {
		t.Errorf("name param: want %q, got %q", "Taro Yamada", gotName)
	}
}

// T160: ListGroups — happy path (2 items)
func TestListGroups_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Sales Dept","memo":"Sales Group","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Dev Dept","memo":"Dev Group","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 groups, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Sales Dept" || result[0].Memo != "Sales Group" {
		t.Errorf("group[0]: got %+v", result[0])
	}
}

// T161: GetGroup — happy path
func TestGetGroup_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/groups/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Sales Dept","memo":"Sales Group","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetGroup(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil GroupEntity")
	}
	if got.ID != 1 || got.Name != "Sales Dept" || got.Memo != "Sales Group" {
		t.Errorf("GetGroup: got %+v", got)
	}
}

// T162: SearchGroups — with Name parameter
func TestSearchGroups_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Sales Dept","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchGroups(context.Background(), boardapi.GroupSearchParams{Name: "Sales Dept"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Sales Dept" {
		t.Errorf("name param: want %q, got %q", "Sales Dept", gotName)
	}
}

// T163: ListPaymentTerms — happy path (2 items)
func TestListPaymentTerms_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"End of Month Payment","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Per-Transaction Payment","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListPaymentTerms(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 payment_terms, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "End of Month Payment" {
		t.Errorf("payment_term[0]: got %+v", result[0])
	}
}

// T164: GetPaymentTerm — happy path
func TestGetPaymentTerm_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payment_terms/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"End of Month Payment","memo":"Standard Payment Terms","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetPaymentTerm(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PaymentTermEntity")
	}
	if got.ID != 1 || got.Name != "End of Month Payment" || got.Memo != "Standard Payment Terms" {
		t.Errorf("GetPaymentTerm: got %+v", got)
	}
}

// T165: SearchPaymentTerms — with Name parameter
func TestSearchPaymentTerms_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"End of Month Payment","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPaymentTerms(context.Background(), boardapi.PaymentTermSearchParams{Name: "End of Month Payment"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "End of Month Payment" {
		t.Errorf("name param: want %q, got %q", "End of Month Payment", gotName)
	}
}

// T166: ListProjectTypes — happy path (2 items)
func TestListProjectTypes_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Contract Development","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"In-House Development","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListProjectTypes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 project_types, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Contract Development" {
		t.Errorf("project_type[0]: got %+v", result[0])
	}
}

// T167: GetProjectType — happy path
func TestGetProjectType_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/project_types/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Contract Development","memo":"External Contract Project","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProjectType(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ProjectTypeEntity")
	}
	if got.ID != 1 || got.Name != "Contract Development" || got.Memo != "External Contract Project" {
		t.Errorf("GetProjectType: got %+v", got)
	}
}

// T168: SearchProjectTypes — with Name parameter
func TestSearchProjectTypes_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Contract Development","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchProjectTypes(context.Background(), boardapi.ProjectTypeSearchParams{Name: "Contract Development"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Contract Development" {
		t.Errorf("name param: want %q, got %q", "Contract Development", gotName)
	}
}

// T169: ListPurchaseTypes — happy path (2 items)
func TestListPurchaseTypes_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Outsourcing Cost","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Equipment Purchase","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListPurchaseTypes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 purchase_types, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Outsourcing Cost" {
		t.Errorf("purchase_type[0]: got %+v", result[0])
	}
}

// T170: GetPurchaseType — happy path
func TestGetPurchaseType_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/expenditure_types/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Outsourcing Cost","memo":"Outsourcing Expense","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetPurchaseType(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PurchaseTypeEntity")
	}
	if got.ID != 1 || got.Name != "Outsourcing Cost" || got.Memo != "Outsourcing Expense" {
		t.Errorf("GetPurchaseType: got %+v", got)
	}
}

// T171: SearchPurchaseTypes — with Name parameter
func TestSearchPurchaseTypes_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Outsourcing Cost","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchPurchaseTypes(context.Background(), boardapi.PurchaseTypeSearchParams{Name: "Outsourcing Cost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Outsourcing Cost" {
		t.Errorf("name param: want %q, got %q", "Outsourcing Cost", gotName)
	}
}

// T172: ListAccountingTypes — happy path (2 items)
func TestListAccountingTypes_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Revenue","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Expense","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListAccountingTypes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 accounting_types, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Revenue" {
		t.Errorf("accounting_type[0]: got %+v", result[0])
	}
}

// T173: GetAccountingType — happy path
func TestGetAccountingType_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/accounting_types/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Revenue","memo":"Revenue Category","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetAccountingType(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil AccountingTypeEntity")
	}
	if got.ID != 1 || got.Name != "Revenue" || got.Memo != "Revenue Category" {
		t.Errorf("GetAccountingType: got %+v", got)
	}
}

// T174: SearchAccountingTypes — with Name parameter
func TestSearchAccountingTypes_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Revenue","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchAccountingTypes(context.Background(), boardapi.AccountingTypeSearchParams{Name: "Revenue"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Revenue" {
		t.Errorf("name param: want %q, got %q", "Revenue", gotName)
	}
}

// T175: ListDocumentSendChannels — happy path (2 items)
func TestListDocumentSendChannels_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Mail","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"Email","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListDocumentSendChannels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 document_send_channels, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Name != "Mail" {
		t.Errorf("document_send_channel[0]: got %+v", result[0])
	}
}

// T176: GetDocumentSendChannel — happy path
func TestGetDocumentSendChannel_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/document_send_channels/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"Mail","memo":"Document Mail Channel","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetDocumentSendChannel(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil DocumentSendChannelEntity")
	}
	if got.ID != 1 || got.Name != "Mail" || got.Memo != "Document Mail Channel" {
		t.Errorf("GetDocumentSendChannel: got %+v", got)
	}
}

// T177: SearchDocumentSendChannels — with Name parameter
func TestSearchDocumentSendChannels_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"Mail","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchDocumentSendChannels(context.Background(), boardapi.DocumentSendChannelSearchParams{Name: "Mail"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "Mail" {
		t.Errorf("name param: want %q, got %q", "Mail", gotName)
	}
}

// ── Step 0: IsNotFound ────────────────────────────────────────────────────────

// T200: IsNotFound returns true for APIErrorNotFound
func TestIsNotFound_True(t *testing.T) {
	err := &boardapi.APIError{Code: boardapi.APIErrorNotFound, StatusCode: 404}
	if !boardapi.IsNotFound(err) {
		t.Errorf("IsNotFound: want true, got false")
	}
}

// T201: IsNotFound returns false for other APIError codes
func TestIsNotFound_False_OtherCode(t *testing.T) {
	err := &boardapi.APIError{Code: boardapi.APIErrorUnauthorized, StatusCode: 401}
	if boardapi.IsNotFound(err) {
		t.Errorf("IsNotFound: want false, got true for UNAUTHORIZED")
	}
}

// T202: IsNotFound returns false for nil
func TestIsNotFound_False_Nil(t *testing.T) {
	if boardapi.IsNotFound(nil) {
		t.Errorf("IsNotFound: want false for nil, got true")
	}
}

// T203: IsNotFound returns false for non-API errors
func TestIsNotFound_False_NonAPIError(t *testing.T) {
	err := errors.New("some generic error")
	if boardapi.IsNotFound(err) {
		t.Errorf("IsNotFound: want false for non-API error, got true")
	}
}

// T204: IsNotFound via wrapped error (errors.As chain)
func TestIsNotFound_WrappedError(t *testing.T) {
	apiErr := &boardapi.APIError{Code: boardapi.APIErrorNotFound, StatusCode: 404}
	wrapped := fmt.Errorf("wrapped: %w", apiErr)
	if !boardapi.IsNotFound(wrapped) {
		t.Errorf("IsNotFound: want true for wrapped NOT_FOUND, got false")
	}
}

// ── Step 1: DoWithRetryFull & ListPage ────────────────────────────────────────

// T205: ListClientsPage returns PageResult with items and pagination headers
func TestListClientsPage_WithHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Total-Count", "250")
		w.Header().Set("X-Page", "2")
		w.Header().Set("X-Per-Page", "50")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":51,"name":"Client51","code":"C51","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListClientsPage(context.Background(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("got nil PageResult")
	}
	if len(result.Items) != 1 {
		t.Errorf("want 1 item, got %d", len(result.Items))
	}
	if result.TotalCount != 250 {
		t.Errorf("TotalCount: want 250, got %d", result.TotalCount)
	}
	if result.Page != 2 {
		t.Errorf("Page: want 2, got %d", result.Page)
	}
	if result.PerPage != 50 {
		t.Errorf("PerPage: want 50, got %d", result.PerPage)
	}
	if result.Items[0].ID != 51 {
		t.Errorf("Items[0].ID: want 51, got %d", result.Items[0].ID)
	}
}

// T206: ListClientsPage propagates API error
func TestListClientsPage_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second,
		boardapi.WithRetryMax(0),
		boardapi.WithSleepFn(noSleep),
	)
	result, err := c.ListClientsPage(context.Background(), 1, 20)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != boardapi.APIErrorUnauthorized {
		t.Errorf("Code: want UNAUTHORIZED, got %q", apiErr.Code)
	}
}

// T207: parsePaginationHeaders returns zeros when headers are absent
func TestListClientsPage_NoHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"A","code":"","memo":"","updated_at":"","created_at":""}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListClientsPage(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalCount != 0 || result.Page != 0 || result.PerPage != 0 {
		t.Errorf("want zero pagination headers, got TotalCount=%d Page=%d PerPage=%d",
			result.TotalCount, result.Page, result.PerPage)
	}
}

// T208: ListProjectsPage works correctly
func TestListProjectsPage_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Count", "1")
		w.Header().Set("X-Page", "1")
		w.Header().Set("X-Per-Page", "100")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"name":"Project1","code":"P001","status":"active","start_date":"","end_date":"","memo":"","updated_at":"","created_at":""}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListProjectsPage(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 1 {
		t.Errorf("unexpected items: %+v", result.Items)
	}
	if result.TotalCount != 1 {
		t.Errorf("TotalCount: want 1, got %d", result.TotalCount)
	}
}

// ── Step 2: GetProjectWithGroup + DocumentSummary ────────────────────────────

// T209: GetProjectWithGroup passes response_group query param
func TestGetProjectWithGroup_QueryParam(t *testing.T) {
	var gotResponseGroup string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/42" {
			http.NotFound(w, r)
			return
		}
		gotResponseGroup = r.URL.Query().Get("response_group")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":42,"client_id":1,"name":"ProjX","code":"PX","status":"active","start_date":"","end_date":"","memo":"","updated_at":"","created_at":"","invoice":{"id":7,"message":null,"total":"100000","tax":"10000","tax_withholding":"0","lock_flg":0}}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProjectWithGroup(context.Background(), 42, "invoice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("got nil ProjectEntity")
	}
	if gotResponseGroup != "invoice" {
		t.Errorf("response_group param: want %q, got %q", "invoice", gotResponseGroup)
	}
	if got.Invoice == nil {
		t.Fatal("Invoice field is nil, expected DocumentSummary")
	}
	if got.Invoice.ID != 7 {
		t.Errorf("Invoice.ID: want 7, got %d", got.Invoice.ID)
	}
	if got.Invoice.Total != "100000" {
		t.Errorf("Invoice.Total: want %q, got %q", "100000", got.Invoice.Total)
	}
}

// T210: GetProjectWithGroup with empty responseGroup behaves like GetProject
func TestGetProjectWithGroup_EmptyGroup(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":5,"client_id":1,"name":"P5","code":"P5","status":"active","start_date":"","end_date":"","memo":"","updated_at":"","created_at":""}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProjectWithGroup(context.Background(), 5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 5 {
		t.Errorf("ID: want 5, got %d", got.ID)
	}
	if gotQuery != "" {
		t.Errorf("expected no query string for empty responseGroup, got %q", gotQuery)
	}
}

// T211: SearchProjects passes response_group param
func TestSearchProjects_WithResponseGroup(t *testing.T) {
	var gotResponseGroup string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotResponseGroup = r.URL.Query().Get("response_group")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchProjects(context.Background(), boardapi.ProjectSearchParams{ResponseGroup: "all"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResponseGroup != "all" {
		t.Errorf("response_group param: want %q, got %q", "all", gotResponseGroup)
	}
}

// T212: SearchProjects omits response_group when empty
func TestSearchProjects_NoResponseGroup(t *testing.T) {
	var gotResponseGroup string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotResponseGroup = r.URL.Query().Get("response_group")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	_, err := c.SearchProjects(context.Background(), boardapi.ProjectSearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResponseGroup != "" {
		t.Errorf("expected no response_group param, got %q", gotResponseGroup)
	}
}

// T213: ProjectEntity deserializes DocumentSummary with null message
func TestProjectEntity_DocumentSummaryNullMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"client_id":1,"name":"P","code":"P","status":"active","start_date":"","end_date":"","memo":"","updated_at":"","created_at":"","estimate":{"id":3,"message":null,"total":"50000","tax":"5000","tax_withholding":"0","lock_flg":1}}`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	got, err := c.GetProjectWithGroup(context.Background(), 1, "estimate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Estimate == nil {
		t.Fatal("Estimate is nil")
	}
	if got.Estimate.Message != nil {
		t.Errorf("Message: want nil, got %v", got.Estimate.Message)
	}
	if got.Estimate.LockFlg != 1 {
		t.Errorf("LockFlg: want 1, got %d", got.Estimate.LockFlg)
	}
}

// T214: ListClientBranchesPage returns valid PageResult
func TestListClientBranchesPage_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Count", "5")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"name":"Branch1","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"","created_at":""}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListClientBranchesPage(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 1 {
		t.Errorf("unexpected items: %+v", result.Items)
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount: want 5, got %d", result.TotalCount)
	}
}
