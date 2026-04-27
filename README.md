# board

[![CI](https://github.com/youyo/board/actions/workflows/ci.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/ci.yml)
[![Release](https://github.com/youyo/board/actions/workflows/release.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/release.yml)

[日本語版はこちら](README_ja.md)

CLI tool and local MCP server for the [BOARD API](https://api.the-board.jp/v1/). Ships as a single binary `board`.

- **`board api`** — Low-level, API-aligned commands for all 22 resources
- **`board find`** — High-level, LLM-friendly cross-resource search
- **`board mcp serve`** — Local HTTP MCP server (for AI assistants)
- SQLite cache to respect BOARD API rate limits (3,000/day, 3/sec)

## Documentation

- [Installation](docs/installation.md)
- [Getting Started](docs/guides/getting-started.md)
- [MCP Server Guide](docs/guides/mcp-server.md)
- [API Reference](docs/api-reference.md)

## Agent / LLM integration

### Embedded `board docs` subcommand

The `board` binary ships with embedded documentation — no network required:

```sh
board docs                          # show README
board docs --list                   # list all embedded docs
board docs clients                  # extract a resource reference
board docs clients --format json    # machine-readable output for LLMs
board docs --search "Ransack"       # full-text search
```

For the full `--format json` schema and the complete list of embedded docs,
see [API Reference › board docs subcommand](docs/api-reference.md#board-docs-サブコマンド).

### `/board:docs` Claude Code skill

A thin Claude Code skill (`skills/docs/SKILL.md`) wraps the commands above so
AI agents can look up BOARD CLI usage on demand. Register this repository as a
Claude Code plugin (it ships a `.claude-plugin/plugin.json`) to expose the
skill as `/board:docs`. The skill stays minimal by design — it only points the
agent at `board docs`, and the binary remains the single source of truth.

## Installation

### Homebrew (macOS / Linux)

```sh
brew install youyo/tap/board
```

### GitHub Releases

Download the binary for your platform from [Releases](https://github.com/youyo/board/releases) and place it in your `$PATH`.

### Build from source

```sh
git clone https://github.com/youyo/board.git
cd board
mise run build   # requires mise + Go 1.26
```

## Quick Start

```sh
# 1. Configure credentials
board configure

# 2. Search for a client
board find client --name "Acme"

# 3. Find projects for that client
board find project --client-name "Acme"

# 4. Pretty-print output
board find invoice --client-name "Acme" --pretty
```

## CLI Commands

### `board configure`

Interactive setup wizard for credentials and settings.

```sh
board configure              # run wizard
board configure show         # show current config (secrets masked)
board configure list-profiles
board configure use <profile>
board configure current-profile
board configure path
```

### `board api`

Low-level access to the BOARD API. Each resource supports `list` and `get`.

```sh
board api <resource> list
board api <resource> list --name-cont "Acme"           # Ransack-style filter
board api <resource> list --updated-at-gteq 2026-01-01 --show-meta
board api <resource> get --id <ID>
board api <resource> get --id <ID> --show-meta         # include _meta (headers) in output
```

**Available resources** (22):

| Category | Resources |
|----------|-----------|
| Core | `clients`, `client-branches`, `contacts`, `projects`, `project-costs` |
| Documents | `estimates`, `invoices`, `orders`, `deliveries`, `receipts` |
| Vendors | `vendors`, `vendor-branches`, `vendor-contacts`, `purchase-orders`, `payments` |
| Masters | `users`, `groups`, `payment-terms`, `project-types`, `purchase-types`, `accounting-types`, `document-send-channels` |

### `board find`

High-level cross-resource search designed for LLM use. Accepts human-readable filters.

```sh
board find client --name "Acme" --text "keyword"
board find project --client-name "Acme" --status active
board find invoice --client-name "Acme" --status draft
board find vendor --name "Supplier"
board find purchase-order --vendor-name "Supplier" --status open
```

**Available resources** (11): `client`, `project`, `estimate`, `invoice`, `order`, `delivery`, `receipt`, `vendor`, `purchase-order`, `payment`, `user`

**Behavior notes** (v0.7.0+):

- **Disambiguate vs fanout**: `find project` / `find invoice` / `find purchase-order` / `find payment` resolve `--client-name` / `--vendor-name` to a single ID (ambiguity error + up to 5 candidates if multiple matches; supply `--id` to disambiguate). Document tools (`find estimate` / `find order` / `find delivery` / `find receipt`) instead **fanout** — they search across all matching clients/projects. Use `--project-id` to narrow.
- **Status narrowing**: `find project --status` requires another narrowing flag (`--id` / `--client-name` / `--name` / `--text`) to avoid full-scan. `find invoice / purchase-order / payment` accept single `--status` (delegated to BOARD API) but reject `--statuses[]` alone.
- **Enrichment is non-fatal**: `Result.Project` / `Result.Client` / `Result.Vendor` may be `nil` if enrichment fails (warn logged, primary entity still returned). Check for nil in your client code.

See [docs/migration/v0.7.0.md](docs/migration/v0.7.0.md) for the v0.6.0 → v0.7.0 migration guide.

### `board cache`

Manage the local SQLite cache.

```sh
board cache status   # show cache state
board cache expire   # mark cache as expired (triggers refresh on next access)
board cache clear    # delete all cached data
board cache path     # show cache file path
```

### `board mcp serve`

Start a local HTTP MCP server that exposes `board find` as MCP tools.

```sh
board mcp serve                      # default: 127.0.0.1:3100
board mcp serve --host 0.0.0.0 --port 8080
```

### `board completion`

Generate shell completion scripts. Fixed-enum flags (`--response-group`, `--order-status-in`,
`--delivery-status-in`, `--invoice-timing-kbn-in`, `--format`) ship with value completion — press
`<TAB>` after the flag to see candidates (with Japanese descriptions in zsh).

```sh
board completion zsh  | sudo tee /usr/local/share/zsh/site-functions/_board
board completion bash > /etc/bash_completion.d/board
```

See [API Reference › CLI 補完値一覧](docs/api-reference.md#cli-補完値一覧) for the full list of
completion values.

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--profile`, `-p` | (current) | Profile name to use |
| `--pretty` | false | Pretty-print JSON output |
| `--limit` | 0 (unlimited) | Max results to return (0 = no limit) |
| `--refresh` | false | Force cache refresh before reading |
| `--force-refresh` | false | Force full cache refresh |
| `--version` | — | Print version |

## Configuration

Config file is TOML, stored at `~/.config/board/config.toml` (XDG: override with `XDG_CONFIG_HOME`). Run `board configure path` to show the resolved path.

```toml
current_profile = "default"
timezone = "UTC"

[profiles.default]
base_url = "https://api.the-board.jp"
api_key = ""        # x-api-key header
api_token = ""      # Bearer token
daily_auto_refresh = true
request_timeout_seconds = 30
retry_max = 5
pretty_default = false
```

Multiple profiles are supported. Switch with `board configure use <profile>` or `-p <profile>` per command.

## MCP Server

`board mcp serve` starts a local HTTP server implementing the [Model Context Protocol](https://modelcontextprotocol.io/). AI assistants (Claude, etc.) can connect to it and call `board find` operations as tools.

**Example Claude Desktop config** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "board": {
      "command": "board",
      "args": ["mcp", "serve"]
    }
  }
}
```

Available MCP tools mirror `board find` (11 tools): `find_client`, `find_project`, `find_estimate`, `find_invoice`, `find_order`, `find_delivery`, `find_receipt`, `find_vendor`, `find_purchase_order`, `find_payment`, `find_user`. (`find_groups` was removed in v0.7.0; use `board api groups list --name-cont <name>` instead.)

**Maintenance contract search**: For step-by-step searches like "verify a customer's maintenance contract", `find_projects` (MCP) and `board find project` (CLI) accept a `contract_status` alias (`active` / `ended` / `prospect` / `all`) and a `statuses[]` / `--statuses` list. See [docs/usage/maintenance-contract-search.md](docs/usage/maintenance-contract-search.md).

## Architecture

```
CLI / MCP
  → service/api (low-level)  /  service/find (high-level)
  → repository  (cache lookup → refresh decision → API fallback)
  → refresh (daily / delta / force)  +  cache (SQLite)
  → boardapi (HTTP client, auth, retry, pagination)
```

## Testing

Unit tests are run on every change. E2E tests that hit the real BOARD API are gated by the `e2e` build tag and **must be executed per-batch** (one Find method at a time) due to the BOARD API rate limit (3 req/sec, 3000/day).

```bash
# Unit tests (CI default)
go test ./...

# Compile-only check for e2e files (no credentials needed)
go vet -tags e2e ./...
go test -tags e2e -run '^$' ./internal/service/find/ ./internal/mcpserver/

# E2E per-batch (requires BOARD_API_KEY + BOARD_API_TOKEN, do NOT run all at once)
go test -tags e2e -v -count=1 -run TestE2E_FindClient   ./internal/service/find/
go test -tags e2e -v -count=1 -run TestE2E_FindProject  ./internal/service/find/
# … 11 batches for service layer + 1 batch for mcpserver
go test -tags e2e -v -count=1 -run TestE2E_MCPHandler   ./internal/mcpserver/
```

Skipped tests use a unified `[SKIP:category] message` log format (categories: `no-creds`, `no-data`, `cache-warm`, `rate-limit`) so CI/log analysis can grep them. The 41 service-layer + 5 MCP-handler representative cases (down from the legacy 193) intentionally tolerate `[SKIP:no-data]` in environments where vendors/payments/etc. are empty; in such cases the *effective* coverage may fall below 33. See `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md §39` for the rationale.

## License

[MIT](LICENSE)
