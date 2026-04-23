package boardapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newPaginationMockClient returns a boardapi.Client whose HTTP client routes
// every request through rt. Mirrors other *_test.go factories so the tests
// work inside the sandbox where local listeners are disallowed.
func newPaginationMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// jsonRespWithHeaders constructs a 200 OK *http.Response with a JSON body and
// additional headers. Used by ListAllWithResult tests.
func jsonRespWithHeaders(body string, extra http.Header) *http.Response {
	h := http.Header{"Content-Type": []string{"application/json"}}
	for k, vs := range extra {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// U9: ListAllWithResult — 3 ページ取得時、最終ページのヘッダーを Meta として保持する。
//
// per_page=2 でページング、3 ページに跨がる応答を mock する。
// 最終 3 ページ目の X-Total-Count="5" / X-Page="3" などが ListResult.Meta に反映されること。
func TestListAllWithResult_MultiPage_PreservesLastPageHeaders(t *testing.T) {
	page1 := `[{"raw":1},{"raw":2}]`
	page2 := `[{"raw":3},{"raw":4}]`
	page3 := `[{"raw":5}]`
	var pageCount int
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		pageCount++
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			return jsonRespWithHeaders(page1, http.Header{
				"X-Total-Count":         {"5"},
				"X-Page":                {"1"},
				"X-Per-Page":            {"2"},
				"X-Ratelimit-Remaining": {"2999"},
			}), nil
		case "2":
			return jsonRespWithHeaders(page2, http.Header{
				"X-Total-Count":         {"5"},
				"X-Page":                {"2"},
				"X-Per-Page":            {"2"},
				"X-Ratelimit-Remaining": {"2998"},
			}), nil
		case "3":
			return jsonRespWithHeaders(page3, http.Header{
				"X-Total-Count":         {"5"},
				"X-Page":                {"3"},
				"X-Per-Page":            {"2"},
				"X-Ratelimit-Remaining": {"2997"},
				"ETag":                  {`W/"last-page"`},
			}), nil
		}
		t.Fatalf("unexpected page request: %s", page)
		return nil, nil
	})
	client := newPaginationMockClient(rt)

	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := client.NewRequest(ctx, http.MethodGet, "/v1/whatever", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", jitoa(page))
		q.Set("per_page", jitoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}

	result, err := client.ListAllWithResult(context.Background(), makeReq, boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListAllWithResult: %v", err)
	}

	if pageCount != 3 {
		t.Errorf("pageCount = %d, want 3", pageCount)
	}
	if len(result.Items) != 5 {
		t.Errorf("Items count = %d, want 5", len(result.Items))
	}
	// 最終ページ (page=3) のヘッダーが Meta に残っていることを検証
	if result.Meta.TotalCount != 5 {
		t.Errorf("Meta.TotalCount = %d, want 5", result.Meta.TotalCount)
	}
	if result.Meta.Page != 3 {
		t.Errorf("Meta.Page = %d, want 3 (last page)", result.Meta.Page)
	}
	if result.Meta.PerPage != 2 {
		t.Errorf("Meta.PerPage = %d, want 2", result.Meta.PerPage)
	}
	if result.Meta.RateLimitRemaining != 2997 {
		t.Errorf("Meta.RateLimitRemaining = %d, want 2997 (last page)", result.Meta.RateLimitRemaining)
	}
	if result.Meta.ETag != `W/"last-page"` {
		t.Errorf("Meta.ETag = %q, want last-page", result.Meta.ETag)
	}
	// Headers も同じく最終ページのものが残っていること
	if result.Headers == nil {
		t.Fatal("Headers should be non-nil")
	}
	if result.Headers.Get("ETag") != `W/"last-page"` {
		t.Errorf("Headers.ETag = %q, want W/\"last-page\"", result.Headers.Get("ETag"))
	}
}

// U10: ListAllWithResult — ctx cancel で中断すると ctx.Err() が返る。
func TestListAllWithResult_CancelsMidPagination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// 最初のページは成功させ、次のページ要求前に cancel する。
	// makeReq 内で cancel を発火することで、内部の ctx.Done() 分岐に入れる。
	var called int
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		called++
		// page=1 のレスポンス（perPage=2 で 2 要素返す → 次ページを要求させる）
		return jsonRespWithHeaders(`[{"x":1},{"x":2}]`, http.Header{
			"X-Total-Count": {"100"},
		}), nil
	})
	client := newPaginationMockClient(rt)
	makeReq := func(cctx context.Context, page, perPage int) (*http.Request, error) {
		if page == 2 {
			// ページ2のリクエスト作成タイミングで ctx をキャンセルし、
			// ループ先頭の ctx.Done() 判定に入れる。
			cancel()
		}
		req, err := client.NewRequest(cctx, http.MethodGet, "/v1/whatever", nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("page", jitoa(page))
		q.Set("per_page", jitoa(perPage))
		req.URL.RawQuery = q.Encode()
		return req, nil
	}
	_, err := client.ListAllWithResult(ctx, makeReq, boardapi.WithPerPage(2))
	if err == nil {
		t.Fatalf("expected cancel error, got nil (called=%d)", called)
	}
	// 具体的には context.Canceled もしくはそれを内包する APIError が返れば OK。
	if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancel") {
		t.Errorf("expected cancel error, got: %v", err)
	}
}

// ListAllWithResult: 最後のページで len(items) < perPage なら追加リクエストしない。
func TestListAllWithResult_StopsOnShortPage(t *testing.T) {
	var pageCount int
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		pageCount++
		return jsonRespWithHeaders(`[{"only":1}]`, http.Header{
			"X-Total-Count": {"1"},
			"X-Page":        {"1"},
		}), nil
	})
	client := newPaginationMockClient(rt)
	makeReq := func(ctx context.Context, page, perPage int) (*http.Request, error) {
		req, err := client.NewRequest(ctx, http.MethodGet, "/v1/whatever", nil)
		if err != nil {
			return nil, err
		}
		return req, nil
	}
	result, err := client.ListAllWithResult(context.Background(), makeReq, boardapi.WithPerPage(100))
	if err != nil {
		t.Fatalf("ListAllWithResult: %v", err)
	}
	if pageCount != 1 {
		t.Errorf("pageCount = %d, want 1", pageCount)
	}
	if len(result.Items) != 1 {
		t.Errorf("Items count = %d, want 1", len(result.Items))
	}
	if result.Meta.TotalCount != 1 {
		t.Errorf("Meta.TotalCount = %d, want 1", result.Meta.TotalCount)
	}
	// Items は json.RawMessage を運ぶ
	var first map[string]any
	if err := json.Unmarshal(result.Items[0], &first); err != nil {
		t.Fatalf("element not valid JSON: %v", err)
	}
	if _, ok := first["only"]; !ok {
		t.Errorf("want key 'only' in first item, got %v", first)
	}
}

// jitoa: 局所用途の簡単な int→string 変換（strconv.Itoa の薄いラッパ、test only）
func jitoa(n int) string { return strings.TrimSpace(itoaSmall(n)) }

func itoaSmall(n int) string {
	// 極小実装: 負数は来ないので扱わない。
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
