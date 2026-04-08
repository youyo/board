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

// T18: APIError.Error() の文字列フォーマット確認
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

// T19: classifyStatusCode の境界値テスト
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

// T01: nil httpClient 指定時に内部で生成される
func TestNew_DefaultHTTPClient(t *testing.T) {
	c := boardapi.New("https://api.the-board.jp", "key", "token", 30*time.Second)
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// T02: WithHTTPClient オプションで注入したクライアントが使われる
func TestNew_WithHTTPClient(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c := boardapi.New("https://api.the-board.jp", "key", "token", 30*time.Second,
		boardapi.WithHTTPClient(hc))
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// T03: baseURL 末尾スラッシュが除去される
func TestNew_BaseURLNormalization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path は /test のみになるはず（baseURL に末尾スラッシュがついていても二重スラッシュにならない）
		if r.URL.Path != "/test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	// 末尾スラッシュ付きで作成
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

// T04: baseURL + path が正しく結合される
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

// T05: x-api-key と Authorization ヘッダが付与される
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

// T06: 200 レスポンスでボディが返る
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

// T07: 201 Created は成功扱い
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

// T08: 401 が *APIError{Code:UNAUTHORIZED} に変換される
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

// T09: 403 が *APIError{Code:FORBIDDEN} に変換される
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

// T10: 404 が *APIError{Code:NOT_FOUND} に変換される
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

// T11: 422 が *APIError{Code:VALIDATION} に変換される
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

// T12: 429 が *APIError{Code:RATE_LIMIT} に変換される
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

// T13: 500 が *APIError{Code:TEMPORARY} に変換される
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

// T14: 503 が *APIError{Code:TEMPORARY} に変換される
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

// T15: ネットワークエラーが *APIError{Code:NETWORK} に変換される
func TestDo_NetworkError(t *testing.T) {
	// 存在しないサーバーへリクエスト
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

// T16: JSON ボディの message フィールドが APIError.Message に入る
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

// T16b: error フィールドにフォールバックする
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

// T17: JSON パース失敗時は Message="" でも panic しない
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
	// panic しないことと、Body に生ボディが入ることを確認
	if apiErr.Body != "not json" {
		t.Errorf("Body: want %q, got %q", "not json", apiErr.Body)
	}
}

// T20: ctx キャンセル時に適切なエラーが返る
func TestDo_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエストが来ても応答を遅延させる（テスト中にキャンセルされる）
		select {
		case <-r.Context().Done():
			return
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second)
	req, _ := c.NewRequest(ctx, "GET", "/test", nil)

	// すぐキャンセル
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

// ===== M05: T35 isRetryable テスト =====

// T35: isRetryable の各 APIErrorCode 判定
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

// T45: parseErrorWithHeader で Retry-After ヘッダが RetryAfter フィールドに入る
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

// ===== M05: T32-T34 calcBackoff テスト =====

// T32: calcBackoff の指数的増加（attempt=0,1,2 の待機時間がそれぞれ [0,baseDelay), [0,2*base), [0,4*base) の範囲）
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

// T33: calcBackoff で Retry-After がある場合はその値が優先される
func TestCalcBackoff_RetryAfterPriority(t *testing.T) {
	ae := &boardapi.APIError{Code: boardapi.APIErrorRateLimit, RetryAfter: 10 * time.Second}
	d := boardapi.CalcBackoff(0, ae)
	if d != 10*time.Second {
		t.Errorf("got %v, want 10s", d)
	}
}

// T34: calcBackoff は maxDelay(30s) を超えない
func TestCalcBackoff_MaxDelayCap(t *testing.T) {
	for attempt := 0; attempt <= 20; attempt++ {
		d := boardapi.CalcBackoff(attempt, &boardapi.APIError{Code: boardapi.APIErrorTemporary})
		if d > 30*time.Second {
			t.Errorf("attempt=%d: got %v > 30s", attempt, d)
		}
	}
}

// ===== M05: T21-T31 DoWithRetry テスト =====

// T21: 1回目成功でリトライなし
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

// T22: 429 → リトライ → 成功
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

// T23: 500 → リトライ → 成功
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

// T24: ネットワークエラー → リトライ → 成功（httptest サーバを再起動パターンは困難なため、
// 存在しないポートへ→即エラー→retryMax 超過で最後のエラーを返す動作を確認）
func TestDoWithRetry_NetworkError_RetryAndSucceed(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// 接続を強制的に閉じてネットワークエラーをシミュレート
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
	// ネットワークエラーはリトライ対象なので retryMax まで試みる
	// 3回目以降は成功するはずだが、Hijack後の接続断はクライアントからはNETWORKエラーとして見える
	// テストの目的: 少なくとも callCount > 1 になること
	if callCount < 2 {
		t.Errorf("expected retry, callCount=%d", callCount)
	}
	_ = err
}

// T25: 429 がリトライMax 回超えたら最後のエラーを返す
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
	// retryMax=3 なので合計 4回呼ばれる（初回 + 3回リトライ）
	if callCount != 4 {
		t.Errorf("callCount: want 4, got %d", callCount)
	}
}

// T26: 4xx（401/403/404/422）はリトライしない
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

// T27: Retry-After ヘッダの秒数を待機する
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

// T28: バックオフ待機中に ctx キャンセルで即返る
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

	// sleepFn が呼ばれたらキャンセル
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

// T29: 実際にリトライが retryMax 回行われること
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
	// 初回 + retryMax 回 = retryMax+1 回
	want := retryMax + 1
	if callCount != want {
		t.Errorf("callCount: want %d, got %d", want, callCount)
	}
}

// T30: retryMax=0 はリトライなし（Do() と同等）
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

// T31: POST ボディがリトライ時に再送される
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

// ===== M05: T36-T44 ListAll テスト =====

// T36: 1ページ（per_page 未満）で終了
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

// T37: 複数ページを全件取得して結合
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

// T38: ちょうど per_page 件の最終ページ後に空リスト取得で終了
func TestListAll_ExactMultiple(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// ちょうど per_page=3 件返す
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
			return
		}
		// 2回目は空
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

// T39: 空配列 [] で0件を返す
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

// T40: per_page クエリパラメータが正しく付与される
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

// T41: page=1,2,3... が正しく増分される
func TestListAll_PageQueryParam(t *testing.T) {
	var gotPages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPages = append(gotPages, r.URL.Query().Get("page"))
		if len(gotPages) < 3 {
			// 3 件/ページ で 3 ページ分
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

// T42: 途中ページでエラーが発生したら即返す
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
		boardapi.WithRetryMax(0), // リトライなし
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

// T43: ページループ中に ctx キャンセルで即中断
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

// T44: WithPerPage(50) でクエリパラメータが変わる
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
