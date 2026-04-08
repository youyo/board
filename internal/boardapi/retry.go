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

// CalcBackoff はリトライ待機時間を計算する。テストから参照できるよう公開している。
// Retry-After ヘッダがある場合はその値を優先する。
// ない場合は指数バックオフ + full jitter。
func CalcBackoff(attempt int, err error) time.Duration {
	if ae, ok := err.(*APIError); ok && ae.RetryAfter > 0 {
		d := ae.RetryAfter
		if d > retryMaxRetryAfter {
			d = retryMaxRetryAfter
		}
		return d
	}

	// 指数バックオフ: base * 2^attempt
	exp := retryBaseDelay * time.Duration(1<<uint(attempt))
	if exp > retryMaxDelay || exp <= 0 {
		exp = retryMaxDelay
	}
	// Full jitter: [0, exp)
	return time.Duration(rand.Int63n(int64(exp)))
}

// cloneRequest は *http.Request をリトライ可能な形で複製する。
// Body が nil の場合は Body なしでクローン。
// Body がある場合は bytes.Buffer に読み込んでから複製する。
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

// DoWithRetry はリトライ付きでリクエストを実行する。
// context キャンセル時はバックオフ待機中でも即座に返す。
// リトライ非対象エラー（4xx等）は即座に返す。
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
		// context キャンセルをバックオフ待機中も監視
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}
		c.sleepFn(wait)
		// sleepFn 後にもキャンセルを確認
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}
	}
	return nil, lastErr
}
