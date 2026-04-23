package boardapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ListResult is the standardized return type for list / search endpoints.
// It carries the parsed items, parsed metadata, and the raw response headers
// so callers (repository, service, cli, output) can surface pagination info,
// rate limits, and caching hints uniformly.
//
// Headers is intentionally omitted from JSON output (`json:"-"`). Callers that
// want to expose metadata to downstream consumers should rely on Meta.
//
// Meta is serialized under the `_meta` JSON key (leading underscore) so that
// `jq '._meta'` conventions used in the CLI / docs work uniformly across
// resources (see plans/board-phase-l-m50-clients-pilot.md §D4).
type ListResult[T any] struct {
	Items   []T         `json:"items"`
	Meta    ListMeta    `json:"_meta"`
	Headers http.Header `json:"-"`
}

// ItemResult mirrors ListResult for single-item endpoints (Get*).
type ItemResult[T any] struct {
	Item    *T          `json:"item"`
	Meta    ItemMeta    `json:"_meta"`
	Headers http.Header `json:"-"`
}

// ListMeta holds commonly-needed response metadata for list endpoints.
// Field naming uses snake_case to match cli JSON output. All fields are
// `omitempty` so a zero ListMeta marshals as `{}`.
//
// NOTE: Exact BOARD API header names are finalized by the M50 clients pilot.
// M49 extracts values under both `X-Ratelimit-*` and `X-RateLimit-*` spellings
// and falls back to zero values when headers are absent.
//
// The zero value of RateLimitReset (time.Time{}) is suppressed from JSON by
// our MarshalJSON implementation; encoding/json does not treat zero Time as
// empty under `omitempty`, so we route marshaling through a helper.
type ListMeta struct {
	TotalCount         int       `json:"total_count,omitempty"`
	Page               int       `json:"page,omitempty"`
	PerPage            int       `json:"per_page,omitempty"`
	RateLimitRemaining int       `json:"rate_limit_remaining,omitempty"`
	RateLimitLimit     int       `json:"rate_limit_limit,omitempty"`
	RateLimitReset     time.Time `json:"rate_limit_reset,omitempty"`
	RetryAfter         int       `json:"retry_after,omitempty"` // seconds (429 / 503 only)
	ETag               string    `json:"etag,omitempty"`
	LastModified       string    `json:"last_modified,omitempty"`
}

// MarshalJSON renders ListMeta with zero time.Time values suppressed. This
// works around encoding/json's inability to treat `time.Time{}` as empty for
// `omitempty`.
func (m ListMeta) MarshalJSON() ([]byte, error) {
	aux := struct {
		TotalCount         int        `json:"total_count,omitempty"`
		Page               int        `json:"page,omitempty"`
		PerPage            int        `json:"per_page,omitempty"`
		RateLimitRemaining int        `json:"rate_limit_remaining,omitempty"`
		RateLimitLimit     int        `json:"rate_limit_limit,omitempty"`
		RateLimitReset     *time.Time `json:"rate_limit_reset,omitempty"`
		RetryAfter         int        `json:"retry_after,omitempty"`
		ETag               string     `json:"etag,omitempty"`
		LastModified       string     `json:"last_modified,omitempty"`
	}{
		TotalCount:         m.TotalCount,
		Page:               m.Page,
		PerPage:            m.PerPage,
		RateLimitRemaining: m.RateLimitRemaining,
		RateLimitLimit:     m.RateLimitLimit,
		RetryAfter:         m.RetryAfter,
		ETag:               m.ETag,
		LastModified:       m.LastModified,
	}
	if !m.RateLimitReset.IsZero() {
		t := m.RateLimitReset
		aux.RateLimitReset = &t
	}
	return json.Marshal(aux)
}

// ItemMeta holds metadata for single-item endpoints (no pagination fields).
type ItemMeta struct {
	RateLimitRemaining int       `json:"rate_limit_remaining,omitempty"`
	RateLimitLimit     int       `json:"rate_limit_limit,omitempty"`
	RateLimitReset     time.Time `json:"rate_limit_reset,omitempty"`
	ETag               string    `json:"etag,omitempty"`
	LastModified       string    `json:"last_modified,omitempty"`
}

// MarshalJSON renders ItemMeta with zero time.Time values suppressed (see
// ListMeta.MarshalJSON for rationale).
func (m ItemMeta) MarshalJSON() ([]byte, error) {
	aux := struct {
		RateLimitRemaining int        `json:"rate_limit_remaining,omitempty"`
		RateLimitLimit     int        `json:"rate_limit_limit,omitempty"`
		RateLimitReset     *time.Time `json:"rate_limit_reset,omitempty"`
		ETag               string     `json:"etag,omitempty"`
		LastModified       string     `json:"last_modified,omitempty"`
	}{
		RateLimitRemaining: m.RateLimitRemaining,
		RateLimitLimit:     m.RateLimitLimit,
		ETag:               m.ETag,
		LastModified:       m.LastModified,
	}
	if !m.RateLimitReset.IsZero() {
		t := m.RateLimitReset
		aux.RateLimitReset = &t
	}
	return json.Marshal(aux)
}

// parseListMeta extracts ListMeta from HTTP response headers.
// Missing headers default to zero values.
func parseListMeta(h http.Header) ListMeta {
	var m ListMeta
	m.TotalCount = atoiHeader(h, "X-Total-Count")
	m.Page = atoiHeader(h, "X-Page")
	m.PerPage = atoiHeader(h, "X-Per-Page")
	m.RateLimitRemaining = firstAtoiHeader(h, "X-Ratelimit-Remaining", "X-RateLimit-Remaining")
	m.RateLimitLimit = firstAtoiHeader(h, "X-Ratelimit-Limit", "X-RateLimit-Limit")
	if v := firstHeader(h, "X-Ratelimit-Reset", "X-RateLimit-Reset"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			m.RateLimitReset = time.Unix(ts, 0).UTC()
		}
	}
	m.RetryAfter = atoiHeader(h, "Retry-After")
	m.ETag = h.Get("ETag")
	m.LastModified = h.Get("Last-Modified")
	return m
}

// parseItemMeta extracts ItemMeta from HTTP response headers.
func parseItemMeta(h http.Header) ItemMeta {
	var m ItemMeta
	m.RateLimitRemaining = firstAtoiHeader(h, "X-Ratelimit-Remaining", "X-RateLimit-Remaining")
	m.RateLimitLimit = firstAtoiHeader(h, "X-Ratelimit-Limit", "X-RateLimit-Limit")
	if v := firstHeader(h, "X-Ratelimit-Reset", "X-RateLimit-Reset"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			m.RateLimitReset = time.Unix(ts, 0).UTC()
		}
	}
	m.ETag = h.Get("ETag")
	m.LastModified = h.Get("Last-Modified")
	return m
}

func atoiHeader(h http.Header, key string) int {
	if v := h.Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func firstHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func firstAtoiHeader(h http.Header, keys ...string) int {
	if v := firstHeader(h, keys...); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}
