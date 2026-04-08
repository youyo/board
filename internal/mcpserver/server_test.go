package mcpserver

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	s := New(nil)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.mcpServer == nil {
		t.Fatal("mcpServer is nil")
	}
	if s.findSvc != nil {
		t.Fatal("findSvc should be nil when passed nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	s := New(nil, WithName("test-board"), WithVersion("1.2.3"))
	if s == nil {
		t.Fatal("New returned nil")
	}
	// Verify the MCPServer was created (we can't inspect name/version directly
	// from mcp-go's MCPServer, but we verify it doesn't panic).
	if s.mcpServer == nil {
		t.Fatal("mcpServer is nil")
	}
}

func TestServer_MCPServer(t *testing.T) {
	s := New(nil)
	if s.MCPServer() == nil {
		t.Fatal("MCPServer() returned nil")
	}
}

func TestServer_FindService_Nil(t *testing.T) {
	s := New(nil)
	if s.FindService() != nil {
		t.Fatal("FindService() should return nil when created with nil")
	}
}

func TestServer_Addr_BeforeStart(t *testing.T) {
	s := New(nil)
	if addr := s.Addr(); addr != "" {
		t.Fatalf("Addr() before Start should be empty, got %q", addr)
	}
}

func TestServer_Shutdown_BeforeStart(t *testing.T) {
	s := New(nil)
	err := s.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown before Start should not error, got %v", err)
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	s := New(nil)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx, "127.0.0.1", 0)
	}()

	// Wait for server to be ready by polling Addr().
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if addr := s.Addr(); addr != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	addr := s.Addr()
	if addr == "" {
		t.Fatal("server did not start within timeout")
	}

	// Verify the server responds to HTTP requests on the /mcp endpoint.
	resp, err := http.Post("http://"+addr+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	resp.Body.Close()
	// MCP server should respond (likely 400 for empty body, but not connection refused).

	// Trigger graceful shutdown via context cancel.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after cancel within timeout")
	}
}

func TestServer_StartPortConflict(t *testing.T) {
	// Start a server to occupy a port.
	s1 := New(nil)
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		_ = s1.Start(ctx1, "127.0.0.1", 0)
	}()

	// Wait for s1 to be ready.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s1.Addr() != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s1.Addr() == "" {
		t.Fatal("s1 did not start")
	}

	// Try to start another server on the same port -- should fail.
	_, portStr, err := net.SplitHostPort(s1.Addr())
	if err != nil {
		t.Fatalf("failed to parse addr %q: %v", s1.Addr(), err)
	}
	port, _ := strconv.Atoi(portStr)

	s2 := New(nil)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	err = s2.Start(ctx2, "127.0.0.1", port)
	if err == nil {
		t.Fatal("expected error when starting on occupied port")
	}
}
