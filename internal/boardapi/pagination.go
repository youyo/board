package boardapi

import (
	"context"
	"encoding/json"
	"net/http"
)

const defaultPerPage = 100

// PagedRequest is the type of a request factory function used by ListAllWithResult for each page.
// It attaches page and per_page query parameters and returns a *http.Request.
type PagedRequest func(ctx context.Context, page, perPage int) (*http.Request, error)

// ListAllOption is a configuration option for ListAllWithResult.
type ListAllOption func(*listAllConfig)

type listAllConfig struct {
	perPage int
}

// WithPerPage specifies the number of items per page. Default is 100.
func WithPerPage(n int) ListAllOption {
	return func(c *listAllConfig) { c.perPage = n }
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
