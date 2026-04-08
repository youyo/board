package boardapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
