package boardapi

import (
	"context"
	"sync/atomic"
)

// retryCounterKey は retry 計装用 ctx.Value のキー型。export しない。
type retryCounterKey struct{}

// WithRetryCounter は ctx に retry 発動回数のカウンタを注入する。
// PoC 等で「retry が発動したか」を観測するためのテスト専用 instrumentation。
// behavior change なし: 注入されない場合は既存挙動通り。
func WithRetryCounter(parent context.Context) (context.Context, *atomic.Int32) {
	c := &atomic.Int32{}
	return context.WithValue(parent, retryCounterKey{}, c), c
}

// recordRetryAttempt は retry ループ内から呼び、ctx に counter があれば +1 する。
// 注入されていなければ no-op（behavior change なし）。
func recordRetryAttempt(ctx context.Context) {
	if v := ctx.Value(retryCounterKey{}); v != nil {
		if c, ok := v.(*atomic.Int32); ok {
			c.Add(1)
		}
	}
}
