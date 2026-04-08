// Package mcpserver provides a local HTTP MCP server wrapping mark3labs/mcp-go.
// It exposes only high-level find.Service operations as MCP tools.
// Tool definitions are added in subsequent milestones (M36-M38).
package mcpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/youyo/board/internal/service/find"
)

// Option configures the MCP server.
type Option func(*config)

type config struct {
	name    string
	version string
}

// WithName sets the server name reported in MCP initialize.
func WithName(name string) Option {
	return func(c *config) { c.name = name }
}

// WithVersion sets the server version reported in MCP initialize.
func WithVersion(version string) Option {
	return func(c *config) { c.version = version }
}

// Server is the MCP server that wraps mark3labs/mcp-go.
// Create with New(), start with Start(), stop with Shutdown().
type Server struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	findSvc    *find.Service

	mu       sync.Mutex
	listener net.Listener
}

// New creates a new MCP Server.
// findSvc may be nil during initialization; it must be set before tools are called.
func New(findSvc *find.Service, opts ...Option) *Server {
	cfg := config{
		name:    "board",
		version: "dev",
	}
	for _, o := range opts {
		o(&cfg)
	}

	mcpSrv := server.NewMCPServer(cfg.name, cfg.version)

	return &Server{
		mcpServer: mcpSrv,
		findSvc:   findSvc,
	}
}

// MCPServer returns the underlying mcp-go MCPServer.
// Use this to register tools (AddTool, AddTools).
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// FindService returns the find.Service used by tool handlers.
func (s *Server) FindService() *find.Service {
	return s.findSvc
}

// Addr returns the listener address after Start has been called.
// Returns "" if the server has not started.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Start begins serving MCP over StreamableHTTP on host:port.
// It blocks until the context is cancelled or the server encounters a fatal error.
// Use port 0 to let the OS choose an available port (see Addr()).
func (s *Server) Start(ctx context.Context, host string, port int) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Create listener ourselves so we can report the actual address (port 0 case).
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mcpserver: listen %s: %w", addr, err)
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	// Build the StreamableHTTP server with a custom http.Server that uses our listener.
	httpSrv := &http.Server{}
	s.httpServer = server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithStreamableHTTPServer(httpSrv),
	)

	// Wire the handler.
	mux := http.NewServeMux()
	mux.Handle("/mcp", s.httpServer)
	httpSrv.Handler = mux

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := httpSrv.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("mcpserver: shutdown: %w", shutdownErr)
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("mcpserver: serve: %w", err)
	}
}

// Shutdown gracefully stops the server.
// Safe to call even if Start was never called.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	if ln == nil {
		return nil
	}
	// Closing the listener will cause Serve to return.
	return ln.Close()
}
