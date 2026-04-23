package boardapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

const defaultPerPage = 100

// PagedRequest is the type of a request factory function used by ListAll for each page.
// It attaches page and per_page query parameters and returns a *http.Request.
type PagedRequest func(ctx context.Context, page, perPage int) (*http.Request, error)

// ListAllOption is a configuration option for ListAll.
type ListAllOption func(*listAllConfig)

type listAllConfig struct {
	perPage int
}

// WithPerPage specifies the number of items per page. Default is 100.
func WithPerPage(n int) ListAllOption {
	return func(c *listAllConfig) { c.perPage = n }
}

// PageResult holds a single page of results with pagination metadata.
type PageResult[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"total_count"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
}

// parsePaginationHeaders extracts pagination info from response headers.
// X-Total-Count, X-Page, X-Per-Page. Defaults to 0 on parse failure.
func parsePaginationHeaders(h http.Header) (totalCount, page, perPage int) {
	if v, err := strconv.Atoi(h.Get("X-Total-Count")); err == nil {
		totalCount = v
	}
	if v, err := strconv.Atoi(h.Get("X-Page")); err == nil {
		page = v
	}
	if v, err := strconv.Atoi(h.Get("X-Per-Page")); err == nil {
		perPage = v
	}
	return
}

// ListPage fetches a single page of results and returns a PageResult.
func ListPage[T any](c *Client, ctx context.Context, makeReq PagedRequest, page, perPage int) (*PageResult[T], error) {
	req, err := makeReq(ctx, page, perPage)
	if err != nil {
		return nil, err
	}
	body, headers, err := c.DoWithRetryFull(req)
	if err != nil {
		return nil, err
	}
	var items []T
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, &APIError{
			Code:    APIErrorUnknown,
			Message: "ListPage: failed to unmarshal page response: " + err.Error(),
		}
	}
	totalCount, pg, pp := parsePaginationHeaders(headers)
	return &PageResult[T]{
		Items:      items,
		TotalCount: totalCount,
		Page:       pg,
		PerPage:    pp,
	}, nil
}

// ListAll fetches all pages and returns []json.RawMessage.
// Each element corresponds to one element in the top-level JSON array of the API response.
// End condition: number of items in response < perPage.
//
// Deprecated: Prefer ListAllWithResult, which returns both items and the final
// page's response metadata (X-Total-Count / Rate Limit / ETag / Last-Modified).
// ListAll is retained for M49 backward compatibility with the 21 resources not
// yet migrated to ListResult[T]; individual resource migrations happen in
// M50-M56 and the legacy helper is scheduled for removal in M57.
func (c *Client) ListAll(ctx context.Context, makeReq PagedRequest, opts ...ListAllOption) ([]json.RawMessage, error) {
	cfg := &listAllConfig{perPage: defaultPerPage}
	for _, o := range opts {
		o(cfg)
	}

	var all []json.RawMessage
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := makeReq(ctx, page, cfg.perPage)
		if err != nil {
			return nil, err
		}

		body, err := c.DoWithRetry(req)
		if err != nil {
			return nil, err
		}

		var items []json.RawMessage
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, &APIError{
				Code:    APIErrorUnknown,
				Message: "ListAll: failed to unmarshal page response: " + err.Error(),
			}
		}

		all = append(all, items...)

		if len(items) < cfg.perPage {
			break // last page
		}
	}
	return all, nil
}

// ListAllWithResult fetches all pages and returns a *ListResult[json.RawMessage]
// whose Meta and Headers are extracted from the FINAL page's HTTP response.
// Callers typically unmarshal each json.RawMessage into a concrete entity and
// wrap the result in ListResult[Entity] (see boardapi.ListClients for the
// canonical example introduced by M49).
//
// End condition: number of items in response < perPage. ctx cancellation is
// honoured at each page boundary.
func (c *Client) ListAllWithResult(ctx context.Context, makeReq PagedRequest, opts ...ListAllOption) (*ListResult[json.RawMessage], error) {
	cfg := &listAllConfig{perPage: defaultPerPage}
	for _, o := range opts {
		o(cfg)
	}

	var all []json.RawMessage
	var lastHeaders http.Header
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := makeReq(ctx, page, cfg.perPage)
		if err != nil {
			return nil, err
		}

		body, headers, err := c.DoWithRetryFull(req)
		if err != nil {
			return nil, err
		}
		lastHeaders = headers

		var items []json.RawMessage
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, &APIError{
				Code:    APIErrorUnknown,
				Message: "ListAllWithResult: failed to unmarshal page response: " + err.Error(),
			}
		}

		all = append(all, items...)

		if len(items) < cfg.perPage {
			break // last page
		}
	}
	return &ListResult[json.RawMessage]{
		Items:   all,
		Meta:    parseListMeta(lastHeaders),
		Headers: lastHeaders,
	}, nil
}
