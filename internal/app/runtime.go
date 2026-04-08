// Package app provides the DI container and common runtime for the board CLI.
package app

import "context"

// contextKey is the key type for Context values.
type contextKey string

// appKey is the key used to store *App in a Context.
const appKey contextKey = "board_app"

// WithApp stores *App in the Context and returns the updated Context.
// It is stored in PersistentPreRunE and retrieved by subcommands via AppFromContext.
func WithApp(ctx context.Context, a *App) context.Context {
	return context.WithValue(ctx, appKey, a)
}

// AppFromContext retrieves *App from the Context.
// Returns (nil, false) if not stored.
func AppFromContext(ctx context.Context) (*App, bool) {
	a, ok := ctx.Value(appKey).(*App)
	return a, ok
}
