package boardapi

import (
	"context"
	"encoding/json"
	"net/http"
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

// ListAll fetches all pages and returns []json.RawMessage.
// Each element corresponds to one element in the top-level JSON array of the API response.
// End condition: number of items in response < perPage.
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
