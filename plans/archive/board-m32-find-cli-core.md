# M32: find CLI - Client & Project Commands

## Scope

Implement `board find client` and `board find project` CLI commands that wrap the existing `service/find.FindClient` and `service/find.FindProject` methods.

## Architecture

```
CLI (find_client.go / find_project.go)
  → findServiceFromCmd() helper (in find.go)
  → service/find.FindClient / FindProject
  → repository → cache → API
```

### New Files

| File | Purpose |
|------|---------|
| `internal/cli/find.go` | `NewFindCmd()` group + `findServiceFromCmd()` helper |
| `internal/cli/find_client.go` | `NewFindClientCmd()` with --id/--name/--text flags |
| `internal/cli/find_project.go` | `NewFindProjectCmd()` with --id/--name/--client-name/--text/--status flags |
| `internal/cli/find_test.go` | Tests for find command structure, flags, and subcommands |

### Modified Files

| File | Change |
|------|--------|
| `internal/cli/root.go` | Add `NewFindCmd()` to root command |

## Design Decisions

### ADR-005 Compliance: English Messages
All help text, descriptions, and error messages in English per ADR-005.

### Flag Design

**`board find client`**:
- `--id <int>` — Direct lookup by ID (highest priority)
- `--name <string>` — Substring match on client name
- `--text <string>` — Free-text search across name, code, memo (lowest priority)
- At least one of --id, --name, --text must be provided

**`board find project`**:
- `--id <int>` — Direct lookup by ID (highest priority)
- `--client-name <string>` — Resolve client name to projects
- `--name <string>` — Project name substring search
- `--text <string>` — Free-text search (lowest priority)
- `--status <string>` — Additional status filter
- At least one of --id, --client-name, --name, --text, --status must be provided

### Service DI Pattern

Follow the existing `apiServiceFromCmd` pattern in `api.go`:
- `findServiceFromCmd(cmd) (*find.Service, error)` creates a find.Service from AppFromContext
- Maps `app.Repositories` fields to `find.Repos` struct

### ReadOptions / Limit

- Global flags `--refresh`, `--force-refresh`, `--limit`, `--pretty` already on root
- `readOptionsFromCmd(cmd)` reused from api.go
- `prettyFromCmd(cmd)` reused from api.go
- Limit is passed into the Query struct (find layer applies limit, not repo layer)

## TDD Implementation Steps

### Step 1: Red — find_test.go (command structure tests)

Write tests verifying:
1. `NewFindCmd()` exists with Use="find"
2. Has "client" and "project" subcommands
3. `NewFindClientCmd()` has --id, --name, --text flags
4. `NewFindProjectCmd()` has --id, --client-name, --name, --text, --status flags
5. Root command registers "find" subcommand

### Step 2: Green — find.go (command group + helper)

Implement:
- `NewFindCmd()` returning cobra.Command with Use="find"
- `findServiceFromCmd(cmd)` helper mapping app.Repos to find.Repos
- Register `NewFindCmd()` in root.go

### Step 3: Green — find_client.go

Implement `NewFindClientCmd()`:
- Flags: --id, --name, --text
- Validation: at least one flag required
- Build FindClientQuery from flags + readOptionsFromCmd
- Call find.Service.FindClient
- Output via output.Write

### Step 4: Green — find_project.go

Implement `NewFindProjectCmd()`:
- Flags: --id, --client-name, --name, --text, --status
- Validation: at least one flag required
- Build FindProjectQuery from flags + readOptionsFromCmd
- Call find.Service.FindProject
- Output via output.Write

### Step 5: Refactor

- Extract shared validation pattern if applicable
- Verify `go vet ./...` and `go test ./...` pass

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| find.Repos field mapping mismatch | Low | Direct copy from apiServiceFromCmd pattern; compile-time check |
| Flag naming inconsistency with spec | Low | Spec says --customer/--client; we use --client-name matching FindProjectQuery.ClientName |
| Missing validation error case | Low | Explicit "at least one flag" check mirrors service layer validation |

## Success Criteria

- [x] `go test ./...` passes
- [x] `go vet ./...` passes
- [x] `board find client --name "test"` dispatches to FindClient
- [x] `board find project --status "open"` dispatches to FindProject
- [x] All messages in English (ADR-005)
- [x] Follows existing CLI patterns (apiServiceFromCmd, readOptionsFromCmd, prettyFromCmd)
