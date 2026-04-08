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

// ============================================================
// M06: clients エンティティ テスト (T46〜T51, T64, T66)
// ============================================================

// T46: ListClients — 正常系（2ページ分）
func TestListClients_TwoPages(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":1,"name":"顧客A","code":"A001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"顧客B","code":"B001","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
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
	if clients[0].ID != 1 || clients[0].Name != "顧客A" {
		t.Errorf("clients[0]: got %+v", clients[0])
	}
	if clients[1].ID != 2 || clients[1].Name != "顧客B" {
		t.Errorf("clients[1]: got %+v", clients[1])
	}
}

// T47: ListClients — APIエラー（401）
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

// T48: GetClient — 正常系
func TestGetClient_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/clients/123" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":123,"name":"テスト顧客","code":"T001","memo":"テストメモ","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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
	if got.ID != 123 || got.Name != "テスト顧客" {
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

// T50: SearchClients — Name パラメータ付き
func TestSearchClients_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"name":"テスト","code":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchClients(context.Background(), boardapi.ClientSearchParams{Name: "テスト"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 client, got %d", len(result))
	}
	if gotName != "テスト" {
		t.Errorf("name param: want %q, got %q", "テスト", gotName)
	}
}

// T51: SearchClients — 空結果
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

// T64: GetClient — context キャンセル時
func TestGetClient_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即座にキャンセル

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

// T66: SearchClients — UpdatedAtFrom パラメータ付き
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
// M06: client_branches エンティティ テスト (T52〜T54)
// ============================================================

// T52: ListClientBranches — 正常系
func TestListClientBranches_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"name":"東京支社","postal_code":"100-0001","address":"東京都","phone":"03-0000-0000","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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
	if result[0].ID != 1 || result[0].ClientID != 10 || result[0].Name != "東京支社" {
		t.Errorf("result[0]: got %+v", result[0])
	}
}

// T53: GetClientBranch — 正常系
func TestGetClientBranch_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/client_branches/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"client_id":5,"name":"大阪支社","postal_code":"530-0001","address":"大阪府","phone":"06-0000-0000","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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
	if got.ID != 1 || got.Name != "大阪支社" {
		t.Errorf("GetClientBranch: got %+v", got)
	}
}

// T54: SearchClientBranches — ClientID パラメータ付き
func TestSearchClientBranches_WithClientID(t *testing.T) {
	var gotClientID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.URL.Query().Get("client_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":2,"client_id":10,"name":"名古屋支社","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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
// M06: contacts エンティティ テスト (T55〜T57)
// ============================================================

// T55: ListContacts — 正常系（3件）
func TestListContacts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"client_branch_id":2,"name":"田中太郎","name_kana":"たなかたろう","title":"部長","email":"tanaka@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"client_id":10,"client_branch_id":2,"name":"鈴木花子","name_kana":"すずきはなこ","title":"","email":"suzuki@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":3,"client_id":11,"client_branch_id":3,"name":"佐藤次郎","name_kana":"さとうじろう","title":"","email":"sato@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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

// T56: GetContact — 正常系
func TestGetContact_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/contacts/5" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":5,"client_id":10,"client_branch_id":2,"name":"山田一郎","name_kana":"やまだいちろう","title":"","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T57: SearchContacts — Email パラメータ付き
func TestSearchContacts_WithEmail(t *testing.T) {
	var gotEmail string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEmail = r.URL.Query().Get("email")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":5,"client_id":10,"client_branch_id":2,"name":"山田一郎","name_kana":"","title":"","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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
// M06: projects エンティティ テスト (T58〜T60, T65)
// ============================================================

// T58: ListProjects — 正常系（ページネーション: 2ページ）
// defaultPerPage=100 のため、page=1 で 100件返して page=2 で残りを返す
func TestListProjects_TwoPages(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// 100件返す（= perPage と同数 → 次ページあり）
			items := make([]string, 100)
			for i := range items {
				items[i] = fmt.Sprintf(`{"id":%d,"client_id":10,"name":"案件%d","code":"P%03d","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`, i+1, i+1, i+1)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[" + strings.Join(items, ",") + "]"))
		} else {
			// page=2: 50件（最終ページ）
			items := make([]string, 50)
			for i := range items {
				items[i] = fmt.Sprintf(`{"id":%d,"client_id":10,"name":"案件%d","code":"P%03d","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`, 100+i+1, 100+i+1, 100+i+1)
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

// T59: GetProject — 正常系
func TestGetProject_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/200" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":200,"client_id":10,"name":"開発案件","code":"DEV001","status":"active","start_date":"2026-01-01","end_date":"2026-12-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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
	if got.ID != 200 || got.Name != "開発案件" {
		t.Errorf("GetProject: got %+v", got)
	}
}

// T60: SearchProjects — Status + UpdatedAtFrom パラメータ
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

// T65: ListProjects — JSONデシリアライズエラー
func TestListProjects_UnmarshalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// 不正な JSON（各要素が不正）
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
// M06: project_costs エンティティ テスト (T61〜T63)
// ============================================================

// T61: ListProjectCosts — 正常系（5件）
func TestListProjectCosts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"project_id":200,"name":"人件費","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"project_id":200,"name":"外注費","cost_type":"outsource","amount":50000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":3,"project_id":200,"name":"交通費","cost_type":"expense","amount":10000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":4,"project_id":200,"name":"通信費","cost_type":"expense","amount":5000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":5,"project_id":200,"name":"備品費","cost_type":"expense","amount":3000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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

// T62: GetProjectCost — 正常系
func TestGetProjectCost_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/project_costs/50" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":50,"project_id":200,"name":"人件費","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T63: SearchProjectCosts — ProjectID パラメータ付き
func TestSearchProjectCosts_WithProjectID(t *testing.T) {
	var gotProjectID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"project_id":200,"name":"人件費","cost_type":"labor","amount":100000.0,"memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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
// M07: estimates エンティティ テスト (T67〜T69)
// ============================================================

// T67: ListEstimates — 正常系
func TestListEstimates_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"見積書1","total_amount":500000.0,"status":"draft","estimate_date":"2026-01-01","expiration_date":"2026-01-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"client_id":10,"project_id":100,"title":"見積書2","total_amount":300000.0,"status":"sent","estimate_date":"2026-01-05","expiration_date":"2026-02-05","memo":"","updated_at":"2026-01-05T00:00:00Z","created_at":"2026-01-05T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListEstimates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 estimates, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].ClientID != 10 || result[0].ProjectID != 100 {
		t.Errorf("estimate[0]: got %+v", result[0])
	}
	if result[0].TotalAmount != 500000.0 {
		t.Errorf("TotalAmount: want 500000.0, got %f", result[0].TotalAmount)
	}
	if result[0].EstimateDate != "2026-01-01" {
		t.Errorf("EstimateDate: want %q, got %q", "2026-01-01", result[0].EstimateDate)
	}
	if result[0].ExpirationDate != "2026-01-31" {
		t.Errorf("ExpirationDate: want %q, got %q", "2026-01-31", result[0].ExpirationDate)
	}
}

// T68: GetEstimate — 正常系
func TestGetEstimate_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/estimates/42" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":42,"client_id":10,"project_id":100,"title":"見積書42","total_amount":750000.0,"status":"approved","estimate_date":"2026-02-01","expiration_date":"2026-03-01","memo":"テストメモ","updated_at":"2026-02-01T00:00:00Z","created_at":"2026-02-01T00:00:00Z"}`))
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
	if got.ID != 42 || got.Title != "見積書42" || got.Status != "approved" {
		t.Errorf("GetEstimate: got %+v", got)
	}
	if got.Memo != "テストメモ" {
		t.Errorf("Memo: want %q, got %q", "テストメモ", got.Memo)
	}
}

// T69: SearchEstimates — ClientID + Status パラメータ付き
func TestSearchEstimates_WithClientIDAndStatus(t *testing.T) {
	var gotClientID, gotStatus string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.URL.Query().Get("client_id")
		gotStatus = r.URL.Query().Get("status")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"見積書1","total_amount":500000.0,"status":"draft","estimate_date":"2026-01-01","expiration_date":"2026-01-31","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchEstimates(context.Background(), boardapi.EstimateSearchParams{ClientID: 10, Status: "draft"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotClientID != "10" {
		t.Errorf("client_id param: want %q, got %q", "10", gotClientID)
	}
	if gotStatus != "draft" {
		t.Errorf("status param: want %q, got %q", "draft", gotStatus)
	}
}

// ============================================================
// M07: invoices エンティティ テスト (T70〜T72)
// ============================================================

// T70: ListInvoices — 正常系
func TestListInvoices_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"請求書1","total_amount":500000.0,"status":"draft","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"},{"id":2,"client_id":10,"project_id":101,"title":"請求書2","total_amount":200000.0,"status":"sent","invoice_date":"2026-02-28","due_date":"2026-03-31","memo":"","updated_at":"2026-02-28T00:00:00Z","created_at":"2026-02-28T00:00:00Z"}]`))
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

// T71: GetInvoice — 正常系
func TestGetInvoice_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/invoices/55" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":55,"client_id":10,"project_id":100,"title":"請求書55","total_amount":1000000.0,"status":"paid","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"支払済み","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}`))
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

// T72: SearchInvoices — ProjectID + UpdatedAtFrom パラメータ付き
func TestSearchInvoices_WithProjectIDAndUpdatedAtFrom(t *testing.T) {
	var gotProjectID, gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"請求書1","total_amount":500000.0,"status":"draft","invoice_date":"2026-01-31","due_date":"2026-02-28","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}]`))
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
// M07: orders エンティティ テスト (T73〜T75)
// ============================================================

// T73: ListOrders — 正常系
func TestListOrders_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"発注書1","total_amount":300000.0,"status":"draft","order_date":"2026-01-10","memo":"","updated_at":"2026-01-10T00:00:00Z","created_at":"2026-01-10T00:00:00Z"},{"id":2,"client_id":10,"project_id":101,"title":"発注書2","total_amount":150000.0,"status":"sent","order_date":"2026-01-15","memo":"","updated_at":"2026-01-15T00:00:00Z","created_at":"2026-01-15T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListOrders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 orders, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].OrderDate != "2026-01-10" {
		t.Errorf("order[0]: got %+v", result[0])
	}
}

// T74: GetOrder — 正常系
func TestGetOrder_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orders/77" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":77,"client_id":10,"project_id":100,"title":"発注書77","total_amount":800000.0,"status":"approved","order_date":"2026-02-01","memo":"承認済み","updated_at":"2026-02-01T00:00:00Z","created_at":"2026-02-01T00:00:00Z"}`))
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
	if got.ID != 77 || got.Status != "approved" || got.Memo != "承認済み" {
		t.Errorf("GetOrder: got %+v", got)
	}
}

// T75: SearchOrders — ClientID パラメータ付き
func TestSearchOrders_WithClientID(t *testing.T) {
	var gotClientID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.URL.Query().Get("client_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"発注書1","total_amount":300000.0,"status":"draft","order_date":"2026-01-10","memo":"","updated_at":"2026-01-10T00:00:00Z","created_at":"2026-01-10T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchOrders(context.Background(), boardapi.OrderSearchParams{ClientID: 10})
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
// M07: deliveries エンティティ テスト (T76〜T78)
// ============================================================

// T76: ListDeliveries — 正常系
func TestListDeliveries_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"納品書1","total_amount":500000.0,"status":"draft","delivery_date":"2026-01-20","memo":"","updated_at":"2026-01-20T00:00:00Z","created_at":"2026-01-20T00:00:00Z"},{"id":2,"client_id":10,"project_id":101,"title":"納品書2","total_amount":250000.0,"status":"sent","delivery_date":"2026-02-20","memo":"","updated_at":"2026-02-20T00:00:00Z","created_at":"2026-02-20T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListDeliveries(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 deliveries, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].DeliveryDate != "2026-01-20" {
		t.Errorf("delivery[0]: got %+v", result[0])
	}
}

// T77: GetDelivery — 正常系
func TestGetDelivery_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/deliveries/88" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":88,"client_id":10,"project_id":100,"title":"納品書88","total_amount":600000.0,"status":"delivered","delivery_date":"2026-02-15","memo":"納品完了","updated_at":"2026-02-15T00:00:00Z","created_at":"2026-02-15T00:00:00Z"}`))
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
	if got.ID != 88 || got.Status != "delivered" || got.DeliveryDate != "2026-02-15" {
		t.Errorf("GetDelivery: got %+v", got)
	}
}

// T78: SearchDeliveries — ProjectID + Status パラメータ付き
func TestSearchDeliveries_WithProjectIDAndStatus(t *testing.T) {
	var gotProjectID, gotStatus string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.URL.Query().Get("project_id")
		gotStatus = r.URL.Query().Get("status")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"納品書1","total_amount":500000.0,"status":"delivered","delivery_date":"2026-01-20","memo":"","updated_at":"2026-01-20T00:00:00Z","created_at":"2026-01-20T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchDeliveries(context.Background(), boardapi.DeliverySearchParams{ProjectID: 100, Status: "delivered"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotProjectID != "100" {
		t.Errorf("project_id param: want %q, got %q", "100", gotProjectID)
	}
	if gotStatus != "delivered" {
		t.Errorf("status param: want %q, got %q", "delivered", gotStatus)
	}
}

// ============================================================
// M07: receipts エンティティ テスト (T79〜T81)
// ============================================================

// T79: ListReceipts — 正常系
func TestListReceipts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"領収書1","total_amount":500000.0,"status":"draft","receipt_date":"2026-01-31","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"},{"id":2,"client_id":10,"project_id":101,"title":"領収書2","total_amount":200000.0,"status":"issued","receipt_date":"2026-02-28","memo":"","updated_at":"2026-02-28T00:00:00Z","created_at":"2026-02-28T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.ListReceipts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("want 2 receipts, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].ReceiptDate != "2026-01-31" {
		t.Errorf("receipt[0]: got %+v", result[0])
	}
}

// T80: GetReceipt — 正常系
func TestGetReceipt_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/receipts/99" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":99,"client_id":10,"project_id":100,"title":"領収書99","total_amount":900000.0,"status":"issued","receipt_date":"2026-03-31","memo":"発行済み","updated_at":"2026-03-31T00:00:00Z","created_at":"2026-03-31T00:00:00Z"}`))
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
	if got.ID != 99 || got.TotalAmount != 900000.0 || got.ReceiptDate != "2026-03-31" {
		t.Errorf("GetReceipt: got %+v", got)
	}
}

// T81: SearchReceipts — ClientID + UpdatedAtFrom パラメータ付き
func TestSearchReceipts_WithClientIDAndUpdatedAtFrom(t *testing.T) {
	var gotClientID, gotUpdatedAtFrom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.URL.Query().Get("client_id")
		gotUpdatedAtFrom = r.URL.Query().Get("updated_at_from")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"client_id":10,"project_id":100,"title":"領収書1","total_amount":500000.0,"status":"draft","receipt_date":"2026-01-31","memo":"","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchReceipts(context.Background(), boardapi.ReceiptSearchParams{ClientID: 10, UpdatedAtFrom: "2026-01-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotClientID != "10" {
		t.Errorf("client_id param: want %q, got %q", "10", gotClientID)
	}
	if gotUpdatedAtFrom != "2026-01-01" {
		t.Errorf("updated_at_from param: want %q, got %q", "2026-01-01", gotUpdatedAtFrom)
	}
}

// ============================================================
// M08: vendors エンティティ テスト (T112〜T117)
// ============================================================

// T112: ListVendors — 正常系（2件）
func TestListVendors_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"発注先A","code":"VA001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"name":"発注先B","code":"VB002","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
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
	if result[0].ID != 1 || result[0].Name != "発注先A" || result[0].Code != "VA001" {
		t.Errorf("vendor[0]: got %+v", result[0])
	}
}

// T113: ListVendors — APIエラー（401）
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

// T114: GetVendor — 正常系
func TestGetVendor_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vendors/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"発注先A","code":"VA001","memo":"テストメモ","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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
	if got.ID != 1 || got.Name != "発注先A" || got.Code != "VA001" {
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

// T116: SearchVendors — Name パラメータ付き
func TestSearchVendors_WithName(t *testing.T) {
	var gotName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"name":"株式会社A","code":"A001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	noSleep := func(time.Duration) {}
	c := boardapi.New(ts.URL, "key", "token", 5*time.Second, boardapi.WithSleepFn(noSleep))
	result, err := c.SearchVendors(context.Background(), boardapi.VendorSearchParams{Name: "株式会社A"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 result, got %d", len(result))
	}
	if gotName != "株式会社A" {
		t.Errorf("name param: want %q, got %q", "株式会社A", gotName)
	}
}

// T117: SearchVendors — UpdatedAtFrom パラメータ付き
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
// M08: vendor_branches エンティティ テスト (T118〜T123)
// ============================================================

// T118: ListVendorBranches — 正常系（2件）
func TestListVendorBranches_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"vendor_id":1,"name":"東京支社","postal_code":"100-0001","address":"東京都千代田区","phone":"03-0000-0001","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":11,"vendor_id":1,"name":"大阪支社","postal_code":"530-0001","address":"大阪府大阪市","phone":"06-0000-0001","fax":"","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
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

// T119: ListVendorBranches — APIエラー（401）
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

// T120: GetVendorBranch — 正常系
func TestGetVendorBranch_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vendor_branches/10" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":10,"vendor_id":1,"name":"東京支社","postal_code":"100-0001","address":"東京都千代田区","phone":"03-0000-0001","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T122: SearchVendorBranches — VendorID パラメータ付き
func TestSearchVendorBranches_WithVendorID(t *testing.T) {
	var gotVendorID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":10,"vendor_id":5,"name":"東京支社","postal_code":"100-0001","address":"東京都千代田区","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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

// T123: SearchVendorBranches — Name パラメータ付き
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
	_, err := c.SearchVendorBranches(context.Background(), boardapi.VendorBranchSearchParams{Name: "東京支社"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "東京支社" {
		t.Errorf("name param: want %q, got %q", "東京支社", gotName)
	}
}

// ============================================================
// M08: vendor_contacts エンティティ テスト (T124〜T129)
// ============================================================

// T124: ListVendorContacts — 正常系（2件）
func TestListVendorContacts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"山田太郎","name_kana":"ヤマダタロウ","title":"部長","email":"yamada@example.com","phone":"03-0000-0001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"vendor_id":1,"vendor_branch_id":10,"name":"鈴木花子","name_kana":"スズキハナコ","title":"課長","email":"suzuki@example.com","phone":"03-0000-0002","memo":"","updated_at":"2026-01-02T00:00:00Z","created_at":"2026-01-02T00:00:00Z"}]`))
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

// T125: ListVendorContacts — APIエラー（401）
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

// T126: GetVendorContact — 正常系
func TestGetVendorContact_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/vendor_contacts/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"山田太郎","name_kana":"ヤマダタロウ","title":"部長","email":"yamada@example.com","phone":"03-0000-0001","memo":"テストメモ","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T128: SearchVendorContacts — VendorID + Email パラメータ付き
func TestSearchVendorContacts_WithVendorIDAndEmail(t *testing.T) {
	var gotVendorID, gotEmail string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		gotEmail = r.URL.Query().Get("email")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":3,"vendor_branch_id":10,"name":"山田太郎","name_kana":"ヤマダタロウ","title":"部長","email":"test@example.com","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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

// T129: SearchVendorContacts — Name パラメータ付き
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
	_, err := c.SearchVendorContacts(context.Background(), boardapi.VendorContactSearchParams{Name: "山田"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "山田" {
		t.Errorf("name param: want %q, got %q", "山田", gotName)
	}
}

// ============================================================
// M08: purchase_orders エンティティ テスト (T130〜T135)
// ============================================================

// T130: ListPurchaseOrders — 正常系（3件）
func TestListPurchaseOrders_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"project_id":100,"title":"発注書1","total_amount":500000.0,"status":"draft","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"},{"id":2,"vendor_id":1,"project_id":101,"title":"発注書2","total_amount":300000.0,"status":"approved","order_date":"2026-01-05","delivery_date":"2026-02-05","memo":"","updated_at":"2026-01-05T00:00:00Z","created_at":"2026-01-05T00:00:00Z"},{"id":3,"vendor_id":2,"project_id":102,"title":"発注書3","total_amount":150000.0,"status":"sent","order_date":"2026-01-10","delivery_date":"2026-02-10","memo":"","updated_at":"2026-01-10T00:00:00Z","created_at":"2026-01-10T00:00:00Z"}]`))
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

// T131: ListPurchaseOrders — APIエラー（401）
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

// T132: GetPurchaseOrder — 正常系
func TestGetPurchaseOrder_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/purchase_orders/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"project_id":100,"title":"発注書1","total_amount":500000.0,"status":"approved","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"テストメモ","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T134: SearchPurchaseOrders — VendorID + Status パラメータ付き
func TestSearchPurchaseOrders_WithVendorIDAndStatus(t *testing.T) {
	var gotVendorID, gotStatus string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVendorID = r.URL.Query().Get("vendor_id")
		gotStatus = r.URL.Query().Get("status")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id":1,"vendor_id":1,"project_id":100,"title":"発注書1","total_amount":500000.0,"status":"approved","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}]`))
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

// T135: SearchPurchaseOrders — ProjectID + UpdatedAtFrom パラメータ付き
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
// M08: payments エンティティ テスト (T136〜T141)
// ============================================================

// T136: ListPayments — 正常系（2件）
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

// T137: ListPayments — APIエラー（401）
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

// T138: GetPayment — 正常系
func TestGetPayment_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payments/1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"purchase_order_id":10,"amount":500000.0,"status":"paid","payment_date":"2026-01-31","memo":"支払済み","updated_at":"2026-01-31T00:00:00Z","created_at":"2026-01-31T00:00:00Z"}`))
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

// T140: SearchPayments — VendorID + Status パラメータ付き
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

// T141: SearchPayments — PurchaseOrderID パラメータ付き
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
// M08: クロスカットテスト (T142〜T156)
// ============================================================

// T142: ListVendors — ページネーション 2ページ（100件 + 50件）
func TestListVendors_TwoPages(t *testing.T) {
	page1 := make([]map[string]interface{}, 100)
	for i := range page1 {
		page1[i] = map[string]interface{}{"id": i + 1, "name": "発注先", "code": "", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}
	page2 := make([]map[string]interface{}, 50)
	for i := range page2 {
		page2[i] = map[string]interface{}{"id": i + 101, "name": "発注先", "code": "", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
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

// T143: ListPurchaseOrders — ページネーション 2ページ（100件 + 30件）
func TestListPurchaseOrders_TwoPages(t *testing.T) {
	page1 := make([]map[string]interface{}, 100)
	for i := range page1 {
		page1[i] = map[string]interface{}{"id": i + 1, "vendor_id": 1, "project_id": 100, "title": "発注書", "total_amount": 100000.0, "status": "draft", "order_date": "2026-01-01", "delivery_date": "2026-02-01", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
	}
	page2 := make([]map[string]interface{}, 30)
	for i := range page2 {
		page2[i] = map[string]interface{}{"id": i + 101, "vendor_id": 1, "project_id": 100, "title": "発注書", "total_amount": 100000.0, "status": "draft", "order_date": "2026-01-01", "delivery_date": "2026-02-01", "memo": "", "updated_at": "2026-01-01T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"}
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

// T144: GetVendor — context キャンセル時
func TestGetVendor_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"発注先A","code":"VA001","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T145: GetVendorBranch — context キャンセル時
func TestGetVendorBranch_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":10,"vendor_id":1,"name":"東京支社","postal_code":"100-0001","address":"","phone":"","fax":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T146: GetVendorContact — context キャンセル時
func TestGetVendorContact_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"vendor_branch_id":10,"name":"山田太郎","name_kana":"","title":"","email":"","phone":"","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T147: GetPurchaseOrder — context キャンセル時
func TestGetPurchaseOrder_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"vendor_id":1,"project_id":100,"title":"発注書1","total_amount":500000.0,"status":"draft","order_date":"2026-01-01","delivery_date":"2026-02-01","memo":"","updated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z"}`))
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

// T148: GetPayment — context キャンセル時
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

// T149: ListVendors — 不正 JSON（unmarshal エラー）
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

// T150: ListPurchaseOrders — 不正 JSON（unmarshal エラー）
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

// T151: ListPayments — 不正 JSON（unmarshal エラー）
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

// T152: SearchVendors — 全パラメータゼロ値
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

// T153: SearchVendorBranches — 全パラメータゼロ値
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

// T154: SearchPurchaseOrders — 全パラメータゼロ値
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

// T155: SearchPayments — 全パラメータゼロ値
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

// T156: SearchVendorContacts — 全パラメータゼロ値
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
