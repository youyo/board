package boardapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client is an HTTP client for the BOARD API.
type Client struct {
	baseURL    string
	apiKey     string
	apiToken   string
	httpClient *http.Client
	retryMax   int                 // default 5 (0 disables retry)
	sleepFn    func(time.Duration) // injectable for testing (default: time.Sleep)
}

// ClientOption is a configuration option for Client.
type ClientOption func(*Client)

// WithHTTPClient injects a custom *http.Client (primarily for testing).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithRetryMax sets the maximum number of retries. 0 disables retry.
func WithRetryMax(n int) ClientOption {
	return func(c *Client) { c.retryMax = n }
}

// WithSleepFn replaces the sleep function for testing.
// for testing only: do not use in production code.
func WithSleepFn(fn func(time.Duration)) ClientOption {
	return func(c *Client) { c.sleepFn = fn }
}

// New creates a Client.
// baseURL is normalized by trimming trailing slashes.
// If timeout is 0 or negative, defaults to 30s.
func New(baseURL, apiKey, apiToken string, timeout time.Duration, opts ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	c := &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryMax: 5,
		sleepFn:  time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewRequest is a helper that creates a *http.Request with baseURL prepended.
// path should be specified with a leading slash, e.g., "/v1/clients".
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	return http.NewRequestWithContext(ctx, method, url, body)
}

// DoFull executes a request and returns the response body and headers on success.
// Non-2xx responses are returned as *APIError (headers are nil).
// Transport errors are also wrapped as *APIError{Code: APIErrorNetwork}.
func (c *Client) DoFull(req *http.Request) ([]byte, http.Header, error) {
	applyAuthHeaders(req, c.apiKey, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, &APIError{
			Code:    APIErrorNetwork,
			Message: err.Error(),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, nil, &APIError{
			Code:    APIErrorNetwork,
			Message: "failed to read response body: " + err.Error(),
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, resp.Header, nil
	}

	return nil, nil, parseErrorWithHeader(resp, body)
}

// Do executes a request and returns the successful response body.
// Non-2xx responses are returned as *APIError.
// Transport errors are also wrapped as *APIError{Code: APIErrorNetwork}.
func (c *Client) Do(req *http.Request) ([]byte, error) {
	applyAuthHeaders(req, c.apiKey, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &APIError{
			Code:    APIErrorNetwork,
			Message: err.Error(),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, &APIError{
			Code:    APIErrorNetwork,
			Message: "failed to read response body: " + err.Error(),
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}

	return nil, parseErrorWithHeader(resp, body)
}
