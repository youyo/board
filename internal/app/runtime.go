// Package app は board CLI の DI コンテナと共通ランタイムを提供する。
package app

import "context"

// contextKey は Context へのキー型。
type contextKey string

// appKey は Context に *App を格納するキー。
const appKey contextKey = "board_app"

// WithApp は *App を Context に格納して返す。
// PersistentPreRunE で格納し、各サブコマンドが AppFromContext で取り出す。
func WithApp(ctx context.Context, a *App) context.Context {
	return context.WithValue(ctx, appKey, a)
}

// AppFromContext は Context から *App を取り出す。
// 格納されていない場合は (nil, false) を返す。
func AppFromContext(ctx context.Context) (*App, bool) {
	a, ok := ctx.Value(appKey).(*App)
	return a, ok
}
