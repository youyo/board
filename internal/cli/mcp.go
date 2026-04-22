package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/mcpserver"
)

// NewMCPCmd returns the `board mcp` parent command.
func NewMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
		Long:  "Commands for running the local MCP (Model Context Protocol) server.",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	var (
		host string
		port int
	)

	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start the MCP server",
		Long:    "Start a local HTTP MCP server exposing BOARD API search tools.",
		Example: "  board mcp serve\n  board mcp serve --port 8080\n  board mcp serve --host 0.0.0.0 --port 3000",
		RunE: func(cmd *cobra.Command, args []string) error {
			findSvc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			srv := mcpserver.New(findSvc)

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting MCP server on %s:%d...\n", host, port)

			if err := srv.Start(cmd.Context(), host, port); err != nil {
				return fmt.Errorf("mcp serve: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind the MCP server")
	cmd.Flags().IntVar(&port, "port", 3100, "Port to bind the MCP server")

	return cmd
}
