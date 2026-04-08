package app_test

import (
	"context"
	"testing"

	"github.com/youyo/board/internal/app"
)

func TestWithApp_RoundTrip(t *testing.T) {
	a := &app.App{}
	ctx := app.WithApp(context.Background(), a)
	got, ok := app.AppFromContext(ctx)
	if !ok {
		t.Fatal("AppFromContext: expected ok=true, got false")
	}
	if got != a {
		t.Fatalf("AppFromContext: got %p, want %p", got, a)
	}
}

func TestAppFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	got, ok := app.AppFromContext(ctx)
	if ok {
		t.Fatal("AppFromContext: expected ok=false, got true")
	}
	if got != nil {
		t.Fatalf("AppFromContext: expected nil, got %v", got)
	}
}
