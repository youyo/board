package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/mcpserver"
	idproxy "github.com/youyo/idproxy"
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
		host  string
		port  int
		flags mcpAuthFlags
	)

	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start the MCP server",
		Long:    "Start an HTTP MCP server exposing BOARD API search tools. Optionally enable OIDC authentication via idproxy with --auth.",
		Example: "  board mcp serve\n  board mcp serve --port 8080\n  board mcp serve --auth (env: BOARD_MCP_OIDC_ISSUER, ...)",
		RunE: func(cmd *cobra.Command, args []string) error {
			applyMCPEnvFallback(cmd, &flags)
			if err := flags.validate(); err != nil {
				return err
			}

			findSvc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			srv := mcpserver.New(findSvc)

			addr := net.JoinHostPort(host, strconv.Itoa(port))

			mux := http.NewServeMux()
			mux.Handle("/mcp", srv.MCPHTTPHandler())
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			})

			var handler http.Handler = mux
			if flags.Auth {
				authCfg, err := buildAuthConfig(&flags)
				if err != nil {
					return err
				}
				defer func() {
					if authCfg.Store != nil {
						_ = authCfg.Store.Close()
					}
				}()
				authMW, err := idproxy.New(cmd.Context(), authCfg)
				if err != nil {
					return fmt.Errorf("idproxy.New: %w", err)
				}
				// /healthz は認証不要のため top-level に切り出す
				top := http.NewServeMux()
				top.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"status":"ok"}`))
				})
				inner := http.NewServeMux()
				inner.Handle("/mcp", srv.MCPHTTPHandler())
				top.Handle("/", authMW.Wrap(inner))
				handler = top
				_, _ = fmt.Fprintf(cmd.OutOrStderr(), "board MCP server (auth enabled) listening on %s/mcp\n", addr)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting MCP server on %s...\n", addr)
			}

			httpSrv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
			ctx := cmd.Context()
			errCh := make(chan error, 1)
			go func() { errCh <- httpSrv.ListenAndServe() }()

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if shutdownErr := httpSrv.Shutdown(shutdownCtx); shutdownErr != nil {
					return fmt.Errorf("mcp serve: shutdown: %w", shutdownErr)
				}
				return nil
			case err := <-errCh:
				if err != nil && err != http.ErrServerClosed {
					return fmt.Errorf("mcp serve: %w", err)
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind the MCP server")
	cmd.Flags().IntVar(&port, "port", 3100, "Port to bind the MCP server")

	// idproxy / OIDC auth flags. すべて env var (BOARD_MCP_*) 対応。
	cmd.Flags().BoolVar(&flags.Auth, "auth", false, "Enable idproxy OIDC authentication")
	cmd.Flags().StringVar(&flags.ExternalURL, "external-url", "", "External URL for OAuth callbacks (env: BOARD_MCP_EXTERNAL_URL)")
	cmd.Flags().StringVar(&flags.OIDCIssuer, "oidc-issuer", "", "OIDC Issuer URL (env: BOARD_MCP_OIDC_ISSUER)")
	cmd.Flags().StringVar(&flags.OIDCClientID, "oidc-client-id", "", "OIDC Client ID (env: BOARD_MCP_OIDC_CLIENT_ID)")
	cmd.Flags().StringVar(&flags.OIDCClientSecret, "oidc-client-secret", "", "OIDC Client Secret (env: BOARD_MCP_OIDC_CLIENT_SECRET)")
	cmd.Flags().StringVar(&flags.CookieSecret, "cookie-secret", "", "Cookie encryption key (hex, 32+ bytes; env: BOARD_MCP_COOKIE_SECRET)")
	cmd.Flags().StringVar(&flags.AllowedDomains, "allowed-domains", "", "Comma-separated allowed email domains (env: BOARD_MCP_ALLOWED_DOMAINS)")
	cmd.Flags().StringVar(&flags.AllowedEmails, "allowed-emails", "", "Comma-separated allowed email addresses (env: BOARD_MCP_ALLOWED_EMAILS)")
	cmd.Flags().StringVar(&flags.SigningKey, "signing-key", "", "ECDSA P-256 signing key in PEM (env: BOARD_MCP_SIGNING_KEY). Required for --idproxy-store=dynamodb.")
	cmd.Flags().DurationVar(&flags.RefreshTokenTTL, "refresh-token-ttl", 0, "MCP OAuth refresh token TTL (e.g. 720h). 0 = idproxy default (env: BOARD_MCP_REFRESH_TOKEN_TTL)")
	cmd.Flags().StringVar(&flags.IDProxyStore, "idproxy-store", "", "idproxy store type: memory|dynamodb (env: BOARD_MCP_IDPROXY_STORE)")
	cmd.Flags().StringVar(&flags.IDProxyStoreDynamoDBTable, "idproxy-store-dynamodb-table", "", "DynamoDB table for idproxy store (env: BOARD_MCP_IDPROXY_STORE_DYNAMODB_TABLE)")
	cmd.Flags().StringVar(&flags.IDProxyStoreDynamoDBRegion, "idproxy-store-dynamodb-region", "", "AWS region for idproxy DynamoDB store (env: BOARD_MCP_IDPROXY_STORE_DYNAMODB_REGION)")

	return cmd
}

// applyMCPEnvFallback はフラグ未指定時に BOARD_MCP_* 環境変数値で埋める。
// cobra に標準の env binding は無いので明示的に処理する。
func applyMCPEnvFallback(cmd *cobra.Command, f *mcpAuthFlags) {
	envStr := func(name string) string { return os.Getenv(name) }
	setIfFlagUnset := func(flagName, envName string, dst *string) {
		if cmd.Flags().Changed(flagName) {
			return
		}
		if v := envStr(envName); v != "" {
			*dst = v
		}
	}
	if !cmd.Flags().Changed("auth") {
		if v := envStr("BOARD_MCP_AUTH"); v == "true" || v == "1" || v == "yes" {
			f.Auth = true
		}
	}
	setIfFlagUnset("external-url", "BOARD_MCP_EXTERNAL_URL", &f.ExternalURL)
	setIfFlagUnset("oidc-issuer", "BOARD_MCP_OIDC_ISSUER", &f.OIDCIssuer)
	setIfFlagUnset("oidc-client-id", "BOARD_MCP_OIDC_CLIENT_ID", &f.OIDCClientID)
	setIfFlagUnset("oidc-client-secret", "BOARD_MCP_OIDC_CLIENT_SECRET", &f.OIDCClientSecret)
	setIfFlagUnset("cookie-secret", "BOARD_MCP_COOKIE_SECRET", &f.CookieSecret)
	setIfFlagUnset("allowed-domains", "BOARD_MCP_ALLOWED_DOMAINS", &f.AllowedDomains)
	setIfFlagUnset("allowed-emails", "BOARD_MCP_ALLOWED_EMAILS", &f.AllowedEmails)
	setIfFlagUnset("signing-key", "BOARD_MCP_SIGNING_KEY", &f.SigningKey)
	setIfFlagUnset("idproxy-store", "BOARD_MCP_IDPROXY_STORE", &f.IDProxyStore)
	setIfFlagUnset("idproxy-store-dynamodb-table", "BOARD_MCP_IDPROXY_STORE_DYNAMODB_TABLE", &f.IDProxyStoreDynamoDBTable)
	setIfFlagUnset("idproxy-store-dynamodb-region", "BOARD_MCP_IDPROXY_STORE_DYNAMODB_REGION", &f.IDProxyStoreDynamoDBRegion)
	if !cmd.Flags().Changed("refresh-token-ttl") {
		if v := envStr("BOARD_MCP_REFRESH_TOKEN_TTL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				f.RefreshTokenTTL = d
			}
		}
	}
}
