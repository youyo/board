package boardapi

import (
	"context"
	"encoding/json"
	"net/http"
)

const defaultPerPage = 100

// PagedRequest は ListAll が各ページに使うリクエスト生成関数の型。
// page と per_page クエリパラメータを付与して *http.Request を返す。
type PagedRequest func(ctx context.Context, page, perPage int) (*http.Request, error)

// ListAllOption は ListAll の設定オプション。
type ListAllOption func(*listAllConfig)

type listAllConfig struct {
	perPage int
}

// WithPerPage は1ページあたりの件数を指定する。デフォルト 100。
func WithPerPage(n int) ListAllOption {
	return func(c *listAllConfig) { c.perPage = n }
}

// ListAll は全ページを取得して []json.RawMessage を返す。
// 各要素は API レスポンスのトップレベル JSON 配列の1要素に対応する。
// ページの終了条件: レスポンスの件数 < perPage。
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
			break // 最終ページ
		}
	}
	return all, nil
}
