# board

[![CI](https://github.com/youyo/board/actions/workflows/ci.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/ci.yml)
[![Release](https://github.com/youyo/board/actions/workflows/release.yml/badge.svg)](https://github.com/youyo/board/actions/workflows/release.yml)

[日本語版はこちら](README_ja.md)

CLI tool and local MCP server for the [BOARD API](https://api.the-board.jp/v1/). Ships as a single binary `board`.

- **`board api`** — Low-level, API-aligned commands for all 22 resources
- **`board find`** — High-level, LLM-friendly cross-resource search
- **`board mcp serve`** — Local HTTP MCP server (for AI assistants)
- SQLite cache to respect BOARD API rate limits (3,000/day, 3/sec)

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

Low-level access to the BOARD API. Each resource supports `list`, `get`, and `search`.

```sh
board api <resource> list
board api <resource> get --id <ID>
board api <resource> search [flags]
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

**Available resources** (12): `client`, `project`, `estimate`, `invoice`, `order`, `delivery`, `receipt`, `vendor`, `purchase-order`, `payment`, `user`, `group`

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

Generate shell completion scripts.

```sh
board completion zsh  | sudo tee /usr/local/share/zsh/site-functions/_board
board completion bash > /etc/bash_completion.d/board
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--profile`, `-p` | (current) | Profile name to use |
| `--pretty` | false | Pretty-print JSON output |
| `--limit` | 50 | Max results to return |
| `--refresh` | false | Force cache refresh before reading |
| `--force-refresh` | false | Force full cache refresh |
| `--version` | — | Print version |

## Configuration

Config file is TOML, stored at the platform XDG config path (shown by `board configure path`).

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

Available MCP tools mirror `board find`: `find_client`, `find_project`, `find_estimate`, `find_invoice`, `find_order`, `find_delivery`, `find_receipt`, `find_vendor`, `find_purchase_order`, `find_payment`, `find_user`, `find_group`.

## Architecture

```
CLI / MCP
  → service/api (low-level)  /  service/find (high-level)
  → repository  (cache lookup → refresh decision → API fallback)
  → refresh (daily / delta / force)  +  cache (SQLite)
  → boardapi (HTTP client, auth, retry, pagination)
```

## License

[MIT](LICENSE)
