# board

English | [日本語](README_ja.md)

**A Remote MCP server for using BOARD (https://the-board.jp) from AI agents.**
It reads the 22 resources of the BOARD API (`https://api.the-board.jp/v1/`) through **54 tools**.
It runs on Cloudflare Workers + D1 + Hono, and sits behind Cloudflare Access (MCP Server Portal + Managed OAuth).

> A rewrite of the Go CLI + local MCP (v0.11.1). The Go version is kept on the `archive/go-cli` branch and the annotated tag `go-cli-final` (= v0.11.1).
> It is not included in main (read it with `git show go-cli-final:<path>`). CLI-only features (configure / completion / docs / cache path) were not ported.

## Architecture

- Cloudflare Workers + D1 + Hono. No Durable Objects.
- MCP is a stateless implementation using `createMcpHandler` from `agents` (`@modelcontextprotocol/server` v2).
- **Authentication is delegated to Cloudflare Access.** The Worker has no OAuth server. It verifies `Cf-Access-Jwt-Assertion` (signature, iss, aud, exp) on every request except `/healthz`.
- **The BOARD API key / token are Worker secrets shared by everyone** (1 deployment = 1 BOARD account).
- To stay within the BOARD API rate limit (3 requests/second, 3,000 requests/day), entities are cached in D1. The LLM decides freshness with the `refresh` argument.

```
MCP client (Claude, etc.)
   │  OAuth (Access Managed OAuth)
   ▼
┌──────────── Cloudflare ─────────────────────────────────────────────────────┐
│ MCP Server Portal (or MCP server application)                               │
│   │ Cf-Access-Jwt-Assertion                                                  │
│   ▼                                                                          │
│ board (Worker)  /healthz (no auth)  /mcp (JWT check → tools)                 │
│   │  54 tools                                                                │
│   ├─▶ D1: resource_cache / sync_state / document → project reverse lookup    │
│   └─▶ BOARD API (x-api-key + Bearer. serial, retries, 30 s deadline)         │
└──────────────────────────────────────────────────────────────────────────────┘
```

See [docs/adr/](docs/adr/) for the reasons behind the design (the ADRs are written in Japanese).

- [ADR 0001: Shared key + Access Managed OAuth](docs/adr/0001-shared-key-and-access-managed-oauth.md)
- [ADR 0002: D1 cache and resumable refresh](docs/adr/0002-d1-cache-and-resumable-refresh.md)
- [ADR 0003: 30-second call deadline](docs/adr/0003-call-deadline.md)
- [ADR 0004: Reverse lookup from document ID to project ID](docs/adr/0004-document-reverse-lookup.md)
- [ADR 0005: Upstream / downstream deployment model](docs/adr/0005-upstream-downstream-deploy.md)
- [ADR 0006: Including archived and lost records, and invalidating old caches by query version](docs/adr/0006-archive-lost-and-query-version.md)

## Tools (54 tools)

Tool names have no prefix. Through an MCP Server Portal, `board_` is prepended (for example `board_list_clients`).

| Group | Count | Tools |
|---|---|---|
| `server` | 1 | `server_info` |
| `list` | 18 | `list_accounting_types`, `list_client_branches`, `list_clients`, `list_contacts`, `list_document_send_channels`, `list_groups`, `list_invoices`, `list_payment_terms`, `list_payments`, `list_project_costs`, `list_project_types`, `list_projects`, `list_purchase_orders`, `list_purchase_types`, `list_users`, `list_vendor_branches`, `list_vendor_contacts`, `list_vendors` |
| `get` | 22 | `get_accounting_type`, `get_client`, `get_client_branch`, `get_contact`, `get_delivery`, `get_document_send_channel`, `get_estimate`, `get_group`, `get_invoice`, `get_order`, `get_payment`, `get_payment_term`, `get_project`, `get_project_cost`, `get_project_type`, `get_purchase_order`, `get_purchase_type`, `get_receipt`, `get_user`, `get_vendor`, `get_vendor_branch`, `get_vendor_contact` |
| `find` | 11 | `find_clients`, `find_deliveries`, `find_estimates`, `find_invoices`, `find_orders`, `find_payments`, `find_projects`, `find_purchase_orders`, `find_receipts`, `find_users`, `find_vendors` |
| `cache` | 2 | `cache_clear`, `cache_status` |

- **api layer (`list_*` / `get_*`)**: low-level reads that follow the BOARD API. They return BOARD's raw data in snake_case. `show_meta` also returns `_meta` (total count and paging info).
- **find layer (`find_*`)**: high-level search that resolves IDs from names and returns related entities (client, branch, contact, project) alongside. Meant for LLMs.
- **cache layer**: `cache_status` inspects the D1 state (read-only); `cache_clear` deletes the cache (destructive). `cache_expire` was not created (it does not affect reads; see the [plan](plans/board-workers-m06-finish.md), in Japanese).
- Read tools take a `max_response_tokens` argument (a safety net for response size; default 50000, 0 disables it). An oversized response is not truncated; it returns `response_too_large`.
- Errors are returned as `{"schema_version":"1","error":{"code","message","retryable"}}` with `isError: true`.

## Deploy to your own environment

This repository is a **public upstream**. It holds no real values (Worker name, D1 id, domain, Access settings).
**Pull the upstream into your own private repository (the downstream)** and deploy from there to your own Cloudflare account.

```bash
# Create the downstream (details: section 0 of docs/runbook.md)
git clone https://github.com/youyo/board.git my-board && cd my-board
git remote rename origin upstream && git remote add origin <URL of your private repository>
mise install && mise run install

# Write the settings (non-secret values only, in one place)
cp board.config.example.json board.config.json   # fill in the <YOUR_...> values

# First deployment in one command (safe to re-run)
pnpm exec wrangler login
mise run setup
```

- The only setting file the downstream writes is `board.config.json`. `mise run deploy:config` combines it with the upstream `wrangler.jsonc` to generate `deploy/wrangler.jsonc`, which **you commit in the downstream** (`mise run deploy:config:check` detects a forgotten regeneration, hand edits, and unmerged upstream changes).
- `mise run setup` goes through: create D1 → generate → deploy → secrets (`BOARD_API_KEY` / `BOARD_API_TOKEN`, hidden input) → check `/healthz`. The Access application, Managed OAuth, and Portal registration are done in the dashboard; [docs/runbook.md](docs/runbook.md) guides you.
- To take upstream updates: `git fetch upstream && git merge --ff-only upstream/main`. The downstream never edits upstream files.
- To deploy from the downstream's GitHub Actions, see [docs/deploy-ci.md](docs/deploy-ci.md) (template: `deploy.example/deploy.workflow.yml`).
- **A skill that guides the setup**: [.claude/skills/board-setup/SKILL.md](.claude/skills/board-setup/SKILL.md). In the downstream repository, ask Claude Code / Codex to "set up board" and it walks you through step by step (it does not take secret values, and deploys and secret registration happen only after the operator approves).

## Connecting

1. Deploy the Worker following "Deploy to your own environment" above and [docs/runbook.md](docs/runbook.md), and create an Access MCP server application (Managed OAuth).
2. Register the Worker URL `https://<domain>/mcp` in an MCP Server Portal.
3. Connect the MCP client to the Portal URL (Access handles the OAuth login).

To connect an MCP client directly to the Worker, use the URL of the Access MCP server application (the Worker itself does not speak OAuth).

## Prerequisites

- A Cloudflare account (Workers **Paid** plan recommended; see below), Cloudflare Access (Zero Trust), and optionally a custom domain (without one, the Worker is published on workers.dev; see section 5 of [docs/runbook.md](docs/runbook.md)).
- A BOARD API key and token (issued in BOARD's settings screen).
- Local work: [mise](https://mise.jdx.dev/) (`mise.toml` installs node 24 / pnpm 11).

**On the Workers Free plan, a single Worker invocation is limited to 50 D1 queries** (Paid: 1,000; `db.batch()` counts as one; per Cloudflare D1 Limits as of 2026-11).
A full refresh or a reverse-lookup scan uses many queries in one call, so the Paid plan is assumed.

## Limits and usage tips

- **Rate limit**: the BOARD API allows 3 requests/second and 3,000 requests/day. The Worker serializes its BOARD calls and retries 429 / 5xx with a cap, honoring `Retry-After`. Prefer reading from the cache. If several people hit a cache miss at once, the 3 requests/second limit can be exceeded (retries absorb this).
- **30-second deadline**: one tool call is cut off at 30 seconds (so it returns before the MCP client times out). Work that cannot finish in time saves its progress and returns, and the next call continues from there.
- **`refresh`**: an argument of the list and search tools. `none` (default; read the cache as is) / `delta` (fetch the changes since last time) / `full` (re-fetch everything and drop what disappeared).
  The LLM decides which one is needed by looking at the `cache` field in the response (`cached_at` / `full_refreshed_at` / `refresh_state`) or at `cache_status`.
  A resource used for the first time has an empty cache, so repeat `refresh=full` until `refresh_state=complete`. Resources with many records (such as estimates) take several calls.
  `delta` / `full` write to the shared D1 cache and consume the BOARD API daily limit (3,000 requests). They do not modify BOARD data, so the read tools keep their annotation (`readOnlyHint: true`). A client that auto-approves read tools will run a refresh without asking. Specify it only when needed (each tool's `refresh` argument description says so too).
  If another refresh is running at the same time, you get `refresh_in_progress` (retry after `retry_after_seconds` seconds).
- **Input limits**: strings up to 1,000 characters, arrays up to 100 items. Anything larger returns `argument_error`.
- **`limit`**: truncates the number of results (0 or unset means unlimited). When truncated, `limited: true` and `total_before_limit` are added. Use `limit` for searches whose name may match many records.
- **`warnings` in `find_*`**: non-fatal problems, such as failing to fetch auxiliary information or being unable to resolve a document's project ID (the reverse-lookup scan timed out). Re-running often resolves them.
- **Secrets**: the BOARD API key and token and the Access JWT are never written to logs, errors, or MCP responses.

## Development

```bash
mise install          # node / pnpm
mise run install      # dependencies
mise run test         # unit + e2e (workerd, node-e2e). Does not contact the BOARD API
mise run lint         # biome + tsc
mise run deploy:check # generate the deploy config from the dummy values in board.config.example.json and run wrangler deploy --dry-run
mise run test:live    # check against a deployed Worker (not part of test; see the runbook)
```

Working conventions: [CLAUDE.md](CLAUDE.md). Plans: [plans/](plans/). Assumptions that can only be verified on a real environment: [docs/manual-verification.md](docs/manual-verification.md).

## License

MIT (LICENSE).
