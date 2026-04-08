# M36: MCP Tools Definition + Schema

## Overview

Register 12 MCP tools corresponding to the 12 `find.Service` methods. Each tool is defined with a JSON Schema for input parameters using mcp-go v0.47.0's builder API (`mcp.NewTool` + `mcp.WithString`/`mcp.WithNumber`). Tool handlers extract arguments from `CallToolRequest`, build the appropriate `FindXxxQuery`, call the service, and return JSON results as `TextContent`.

## Architecture

```
internal/mcpserver/
  server.go          (existing - M35)
  server_test.go     (existing - M35)
  tools.go           (NEW - tool definitions + RegisterTools function)
  tools_test.go      (NEW - tests)
```

### Key Design Decisions

1. **Single `tools.go` file**: All 12 tool definitions in one file. Each tool is a function returning `server.ServerTool`. A `RegisterTools(srv *Server)` function calls `srv.MCPServer().AddTools(...)` to register all tools at once.

2. **Tool naming**: `find_clients`, `find_projects`, `find_estimates`, `find_invoices`, `find_orders`, `find_deliveries`, `find_receipts`, `find_vendors`, `find_purchase_orders`, `find_payments`, `find_users`, `find_groups` (verb + plural noun per spec section 23.3).

3. **Input parameters**: All optional. Use mcp-go's `mcp.WithString`/`mcp.WithNumber` builder for each field. No `Required()` on any property since all search parameters are optional.

4. **Output**: JSON-serialized `[]XxxResult` as `mcp.TextContent`. Empty array `[]` for no results. Errors returned as `CallToolResult` with `IsError: true`.

5. **ReadOptions**: Not exposed via MCP. Tools use `repository.ReadOptions{}` (zero value = default refresh behavior).

6. **Annotations**: All tools are read-only (`ReadOnlyHint: true`, `DestructiveHint: false`).

## Tool Definitions

### Category 1: Simple (id, name, text, limit)
- **find_clients**: Search clients with branches and contacts
- **find_vendors**: Search vendors with branches and contacts
- **find_users**: Search users
- **find_groups**: Search groups

### Category 2: Project
- **find_projects**: id, client_name, name, text, status, limit

### Category 3: Client-doc (id, client_name, project_name, text, status, limit)
- **find_estimates**, **find_invoices**, **find_orders**, **find_deliveries**, **find_receipts**

### Category 4: Vendor-doc
- **find_purchase_orders**: id, vendor_name, project_name, text, status, limit

### Category 5: Payment
- **find_payments**: id, vendor_name, purchase_order_id, text, status, limit

## Implementation Details

### Helper functions

```go
func getStringArg(req mcp.CallToolRequest, key string) string
func getIntArg(req mcp.CallToolRequest, key string) int
func marshalResult(v any) (*mcp.CallToolResult, error)
func errorResult(err error) *mcp.CallToolResult
```

### Tool builder pattern (each tool returns server.ServerTool)

```go
func findClientsTool(srv *Server) server.ServerTool {
    return server.ServerTool{
        Tool: mcp.NewTool("find_clients",
            mcp.WithDescription("..."),
            mcp.WithNumber("id", mcp.Description("Client ID for direct lookup")),
            mcp.WithString("name", mcp.Description("Substring match on client name")),
            mcp.WithString("text", mcp.Description("Free-text search")),
            mcp.WithNumber("limit", mcp.Description("Max results"), mcp.DefaultNumber(50)),
            mcp.WithToolAnnotation(mcp.ToolAnnotation{
                ReadOnlyHint:    mcp.ToBoolPtr(true),
                DestructiveHint: mcp.ToBoolPtr(false),
            }),
        ),
        Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            results, err := srv.FindService().FindClient(ctx, find.FindClientQuery{...})
            if err != nil { return errorResult(err), nil }
            return marshalResult(results)
        },
    }
}
```

### Integration: Call RegisterTools(s) at end of New()

## TDD Plan

### Red Phase
1. TestRegisterTools_AllToolsRegistered — 12 tools present with correct names
2. TestToolDefinitions_Schema — each tool has correct input schema properties
3. TestGetStringArg / TestGetIntArg — helper edge cases (missing, float64, string)
4. TestMarshalResult / TestErrorResult — output formatting

### Green Phase
1. Implement helpers: getStringArg, getIntArg, marshalResult, errorResult
2. Implement all 12 findXxxTool functions
3. Implement RegisterTools
4. Wire into New()

### Refactor Phase
- Run go test ./..., go vet ./..., gofmt -s -w .

## Implementation Steps

1. Create `internal/mcpserver/tools_test.go` with Red tests
2. Create `internal/mcpserver/tools.go` with helpers + 12 tools + RegisterTools
3. Update `server.go` to call `RegisterTools(s)` in `New()`
4. Run tests until Green
5. Run full suite: `go test ./...`, `go vet ./...`, `gofmt -s -w .`
6. Git commit

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Number args from JSON as float64 | High | getIntArg handles float64 -> int |
| mcp-go API mismatch | Low | Verified from v0.47.0 source |
| Existing tests break | Low | RegisterTools with nil findSvc is safe |

## Success Criteria

- [ ] 12 tools registered via ListTools()
- [ ] Correct JSON Schema per tool (all params optional, correct types)
- [ ] Helpers handle edge cases
- [ ] All tests pass: `go test ./...`
- [ ] `go vet ./...` clean
