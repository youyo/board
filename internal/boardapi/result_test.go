package boardapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// U7: parseListMeta（非公開関数だが挙動は ListResult を通じて検証）は
//
//	X-Total-Count / X-Page / X-Per-Page / Retry-After を ListMeta に正しく写す。
//
// U8: Rate Limit 名揺れ両方対応（X-Ratelimit-Remaining / X-RateLimit-Remaining いずれでも）。
//
// 非公開関数のテストは export_test.go 経由で露出するため、本テストではそれを使う。
func TestParseListMeta_TotalCountPageRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("X-Total-Count", "299")
	h.Set("X-Page", "3")
	h.Set("X-Per-Page", "50")
	h.Set("Retry-After", "10")
	h.Set("ETag", `W/"abc123"`)
	h.Set("Last-Modified", "Wed, 01 Apr 2026 12:00:00 GMT")

	m := boardapi.ParseListMetaForTest(h)
	if m.TotalCount != 299 {
		t.Errorf("TotalCount: want 299, got %d", m.TotalCount)
	}
	if m.Page != 3 {
		t.Errorf("Page: want 3, got %d", m.Page)
	}
	if m.PerPage != 50 {
		t.Errorf("PerPage: want 50, got %d", m.PerPage)
	}
	if m.RetryAfter != 10 {
		t.Errorf("RetryAfter: want 10, got %d", m.RetryAfter)
	}
	if m.ETag != `W/"abc123"` {
		t.Errorf("ETag: want W/\"abc123\", got %q", m.ETag)
	}
	if !strings.Contains(m.LastModified, "Apr 2026") {
		t.Errorf("LastModified: want contains 'Apr 2026', got %q", m.LastModified)
	}
}

func TestParseListMeta_RateLimitHeaderCaseVariants(t *testing.T) {
	// ケース1: "X-Ratelimit-*" 形式
	h1 := http.Header{}
	h1.Set("X-Ratelimit-Remaining", "99")
	h1.Set("X-Ratelimit-Limit", "3000")
	m1 := boardapi.ParseListMetaForTest(h1)
	if m1.RateLimitRemaining != 99 {
		t.Errorf("Ratelimit (lower): want 99, got %d", m1.RateLimitRemaining)
	}
	if m1.RateLimitLimit != 3000 {
		t.Errorf("Ratelimit limit (lower): want 3000, got %d", m1.RateLimitLimit)
	}

	// ケース2: "X-RateLimit-*" 形式（大文字キャメル）
	h2 := http.Header{}
	h2.Set("X-RateLimit-Remaining", "42")
	h2.Set("X-RateLimit-Limit", "1000")
	m2 := boardapi.ParseListMetaForTest(h2)
	if m2.RateLimitRemaining != 42 {
		t.Errorf("RateLimit (camel): want 42, got %d", m2.RateLimitRemaining)
	}
	if m2.RateLimitLimit != 1000 {
		t.Errorf("RateLimit limit (camel): want 1000, got %d", m2.RateLimitLimit)
	}
}

func TestParseListMeta_RateLimitResetUnixTimestamp(t *testing.T) {
	h := http.Header{}
	// 2026-04-23T00:00:00Z の epoch 秒
	h.Set("X-Ratelimit-Reset", "1776643200")
	m := boardapi.ParseListMetaForTest(h)
	if m.RateLimitReset.IsZero() {
		t.Fatal("RateLimitReset: want non-zero, got zero")
	}
	if m.RateLimitReset.Year() != 2026 {
		t.Errorf("RateLimitReset: want year 2026, got %d", m.RateLimitReset.Year())
	}
}

func TestParseListMeta_EmptyHeadersYieldsZeroMeta(t *testing.T) {
	m := boardapi.ParseListMetaForTest(http.Header{})
	if m.TotalCount != 0 || m.Page != 0 || m.PerPage != 0 {
		t.Errorf("empty headers: want all zero, got %+v", m)
	}
	if m.ETag != "" || m.LastModified != "" {
		t.Errorf("empty headers: want empty strings, got %+v", m)
	}
	if !m.RateLimitReset.IsZero() {
		t.Errorf("empty headers: want zero time, got %v", m.RateLimitReset)
	}
}

// U11: ListResult marshal: Headers は json:"-" なので JSON 出力に含まれない。
func TestListResult_MarshalJSON_HeadersExcluded(t *testing.T) {
	res := boardapi.ListResult[string]{
		Items: []string{"a", "b"},
		Meta:  boardapi.ListMeta{TotalCount: 10, Page: 1, PerPage: 50},
		Headers: http.Header{
			"X-Internal": {"should not leak"},
		},
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"items"`) {
		t.Errorf("want items key, got %s", s)
	}
	if !strings.Contains(s, `"meta"`) {
		t.Errorf("want meta key, got %s", s)
	}
	if strings.Contains(s, "headers") || strings.Contains(s, "X-Internal") || strings.Contains(s, "should not leak") {
		t.Errorf("Headers must be excluded from JSON, got %s", s)
	}
}

// U12: ListMeta の各フィールドは omitempty。すべてゼロ値の Meta は `{}`。
func TestListMeta_OmitemptyAllZeros(t *testing.T) {
	b, err := json.Marshal(boardapi.ListMeta{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(b); got != `{}` {
		t.Errorf("empty Meta: want `{}`, got %s", got)
	}
}

// 追加: RateLimitReset が omitempty で非ゼロなら出る、ゼロなら出ない。
func TestListMeta_RateLimitResetOmitemptyWhenZero(t *testing.T) {
	// ゼロ値
	b, err := json.Marshal(boardapi.ListMeta{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "rate_limit_reset") {
		t.Errorf("zero time should be omitted, got %s", string(b))
	}
	// 非ゼロ値
	b2, err := json.Marshal(boardapi.ListMeta{RateLimitReset: time.Unix(1776643200, 0).UTC()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b2), "rate_limit_reset") {
		t.Errorf("non-zero time should appear, got %s", string(b2))
	}
}

// 追加: ItemMeta も同様に正しく抽出される。
func TestParseItemMeta_Basic(t *testing.T) {
	h := http.Header{}
	h.Set("X-Ratelimit-Remaining", "42")
	h.Set("X-Ratelimit-Limit", "3000")
	h.Set("ETag", `"item-tag"`)
	m := boardapi.ParseItemMetaForTest(h)
	if m.RateLimitRemaining != 42 {
		t.Errorf("RateLimitRemaining: want 42, got %d", m.RateLimitRemaining)
	}
	if m.RateLimitLimit != 3000 {
		t.Errorf("RateLimitLimit: want 3000, got %d", m.RateLimitLimit)
	}
	if m.ETag != `"item-tag"` {
		t.Errorf("ETag: want \"item-tag\", got %q", m.ETag)
	}
}
