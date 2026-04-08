package boardapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client は BOARD API への HTTP クライアント。
type Client struct {
	baseURL    string
	apiKey     string
	apiToken   string
	httpClient *http.Client
	retryMax   int                  // デフォルト 5（0 でリトライ無効）
	sleepFn    func(time.Duration)  // テスト差し替え用（デフォルト time.Sleep）
}

// ClientOption は Client の設定オプション。
type ClientOption func(*Client)

// WithHTTPClient はカスタム *http.Client を注入する（主にテスト用途）。
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithRetryMax はリトライ最大回数を設定する。0 でリトライ無効。
func WithRetryMax(n int) ClientOption {
	return func(c *Client) { c.retryMax = n }
}

// WithSleepFn はテスト用にスリープ関数を差し替える。
// for testing only: 本番コードでは使用しないこと。
func WithSleepFn(fn func(time.Duration)) ClientOption {
	return func(c *Client) { c.sleepFn = fn }
}

// New は Client を生成する。
// baseURL は末尾スラッシュを正規化する。
// timeout が 0 以下の場合はデフォルト 30s を使う。
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

// NewRequest は baseURL を付与した *http.Request を生成するヘルパー。
// path は "/v1/clients" のように先頭スラッシュ付きで指定する。
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path
	return http.NewRequestWithContext(ctx, method, url, body)
}

// Do はリクエストを実行し、成功レスポンスのボディを返す。
// 2xx 以外は *APIError として返す。
// transport エラーも *APIError{Code: APIErrorNetwork} にラップして返す。
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{
			Code:    APIErrorNetwork,
			Message: "failed to read response body: " + err.Error(),
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}

	return nil, parseError(resp.StatusCode, body)
}
