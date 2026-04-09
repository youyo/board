package boardapi

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"time"
)

const (
	retryBaseDelay     = 500 * time.Millisecond
	retryMaxDelay      = 30 * time.Second
	retryMaxRetryAfter = 60 * time.Second
)

// CalcBackoff calculates the retry wait duration. Exported so it can be referenced from tests.
// If a Retry-After header is present, its value takes priority.
// Otherwise, uses exponential backoff with full jitter.
func CalcBackoff(attempt int, err error) time.Duration {
	if ae, ok := err.(*APIError); ok && ae.RetryAfter > 0 {
		d := ae.RetryAfter
		if d > retryMaxRetryAfter {
			d = retryMaxRetryAfter
		}
		return d
	}

	// exponential backoff: base * 2^attempt
	exp := retryBaseDelay * time.Duration(1<<uint(attempt))
	if exp > retryMaxDelay || exp <= 0 {
		exp = retryMaxDelay
	}
	// Full jitter: [0, exp)
	return time.Duration(rand.Int63n(int64(exp)))
}

// cloneRequest clones a *http.Request in a retryable form.
// If Body is nil, clones without a Body.
// If Body is present, reads it into a bytes.Buffer before cloning.
func cloneRequest(req *http.Request) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return cloned, nil
	}
	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, &APIError{Code: APIErrorNetwork, Message: "cloneRequest: " + err.Error()}
	}
	req.Body = io.NopCloser(bytes.NewReader(buf))
	cloned.Body = io.NopCloser(bytes.NewReader(buf))
	cloned.ContentLength = req.ContentLength
	return cloned, nil
}

// DoWithRetry executes a request with retry support.
// Returns immediately on context cancellation even during backoff.
// Returns immediately for non-retryable errors (4xx, etc.).
func (c *Client) DoWithRetry(req *http.Request) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		cloned, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}
		body, err := c.Do(cloned)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
		if attempt == c.retryMax {
			break
		}
		wait := CalcBackoff(attempt, err)
		// monitor context cancellation during backoff
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}
		c.sleepFn(wait)
		// also check cancellation after sleepFn
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}
	}
	return nil, lastErr
}

// DoWithRetryFull executes a request with retry support and returns response headers.
// Returns immediately on context cancellation even during backoff.
// Returns immediately for non-retryable errors (4xx, etc.).
func (c *Client) DoWithRetryFull(req *http.Request) ([]byte, http.Header, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		cloned, err := cloneRequest(req)
		if err != nil {
			return nil, nil, err
		}
		body, headers, err := c.DoFull(cloned)
		if err == nil {
			return body, headers, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, nil, err
		}
		if attempt == c.retryMax {
			break
		}
		wait := CalcBackoff(attempt, err)
		// monitor context cancellation during backoff
		select {
		case <-req.Context().Done():
			return nil, nil, req.Context().Err()
		default:
		}
		c.sleepFn(wait)
		// also check cancellation after sleepFn
		select {
		case <-req.Context().Done():
			return nil, nil, req.Context().Err()
		default:
		}
	}
	return nil, nil, lastErr
}
