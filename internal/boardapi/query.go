package boardapi

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// QueryBuilder wraps url.Values with BOARD API-specific semantics:
//   - skips zero / empty values (Go idiom)
//   - supports `_in[]` repeated-key array parameters (Ransack)
//   - supports bool → "0" / "1" for BOARD's `*_flg` fields
//   - supports `response_group` string
//
// QueryBuilder is introduced by M49 and expected to drive all BOARD API
// request parameter construction from M50 onward. It is intentionally
// method-chain friendly.
type QueryBuilder struct {
	v url.Values
}

// NewQueryBuilder returns an empty builder.
func NewQueryBuilder() *QueryBuilder { return &QueryBuilder{v: url.Values{}} }

// Page sets `page` and `per_page`. Zero / negative values are skipped.
func (q *QueryBuilder) Page(page, perPage int) *QueryBuilder {
	if page > 0 {
		q.v.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		q.v.Set("per_page", strconv.Itoa(perPage))
	}
	return q
}

// StrEq sets `<field>_eq=<value>` when value is non-empty.
func (q *QueryBuilder) StrEq(field, value string) *QueryBuilder {
	if value != "" {
		q.v.Set(field+"_eq", value)
	}
	return q
}

// StrCont sets `<field>_cont=<value>` (partial-match / substring filter) when
// value is non-empty. BOARD API's Ransack-based `_cont` matcher is the
// primary means of name-like filtering.
func (q *QueryBuilder) StrCont(field, value string) *QueryBuilder {
	if value != "" {
		q.v.Set(field+"_cont", value)
	}
	return q
}

// IntEq sets `<field>_eq=<value>` when value != 0.
func (q *QueryBuilder) IntEq(field string, value int) *QueryBuilder {
	if value != 0 {
		q.v.Set(field+"_eq", strconv.Itoa(value))
	}
	return q
}

// IntIn sets repeated `<field>_in[]=<v>` parameters. Skips empty slice.
func (q *QueryBuilder) IntIn(field string, values []int) *QueryBuilder {
	for _, v := range values {
		q.v.Add(field+"_in[]", strconv.Itoa(v))
	}
	return q
}

// StrIn sets repeated `<field>_in[]=<v>` parameters. Skips empty values.
func (q *QueryBuilder) StrIn(field string, values []string) *QueryBuilder {
	for _, v := range values {
		if v != "" {
			q.v.Add(field+"_in[]", v)
		}
	}
	return q
}

// DateGteq sets `<field>_gteq=<value>` (≥). Accepts `yyyy-MM-dd` or
// `yyyy-MM-dd HH:mm:ss` — caller's responsibility to pick the right form.
func (q *QueryBuilder) DateGteq(field, value string) *QueryBuilder {
	if value != "" {
		q.v.Set(field+"_gteq", value)
	}
	return q
}

// DateLteq sets `<field>_lteq=<value>` (≤).
func (q *QueryBuilder) DateLteq(field, value string) *QueryBuilder {
	if value != "" {
		q.v.Set(field+"_lteq", value)
	}
	return q
}

// Flg01 sets `<field>=0` or `<field>=1`. A nil pointer means "do not send",
// which is how BOARD API distinguishes "filter disabled" from "filter = 0".
func (q *QueryBuilder) Flg01(field string, value *bool) *QueryBuilder {
	if value == nil {
		return q
	}
	if *value {
		q.v.Set(field, "1")
	} else {
		q.v.Set(field, "0")
	}
	return q
}

// Tags sets repeated `tags[]=<v>` pairs for BOARD's tag filter.
// Empty values are skipped.
func (q *QueryBuilder) Tags(values []string) *QueryBuilder {
	for _, v := range values {
		if v != "" {
			q.v.Add("tags[]", v)
		}
	}
	return q
}

// ResponseGroup sets `response_group=<value>` (e.g. "small" / "large").
func (q *QueryBuilder) ResponseGroup(value string) *QueryBuilder {
	if value != "" {
		q.v.Set("response_group", value)
	}
	return q
}

// Set attaches a custom key-value pair (escape hatch for cases not covered
// by typed helpers). Empty values are skipped.
func (q *QueryBuilder) Set(key, value string) *QueryBuilder {
	if value != "" {
		q.v.Set(key, value)
	}
	return q
}

// Encode returns the canonical query string, suitable for req.URL.RawQuery.
func (q *QueryBuilder) Encode() string { return q.v.Encode() }

// Raw returns the underlying url.Values for advanced composition.
func (q *QueryBuilder) Raw() url.Values { return q.v }

// Debug formats the query as a human-readable string (ordered by key, values
// joined with "|") primarily for logging. The output is NOT URL-encoded.
func (q *QueryBuilder) Debug() string {
	parts := make([]string, 0, len(q.v))
	for k, vs := range q.v {
		parts = append(parts, fmt.Sprintf("%s=%s", k, strings.Join(vs, "|")))
	}
	return strings.Join(parts, "&")
}
