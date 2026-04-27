package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// RegisterTools registers all 11 MCP tool definitions with real handlers.
// find_groups は ADR-001 (Group 削除確定) により削除（N07b）。
// N08: 全 tool description / property description を LLM 向けに刷新。
//   - disambiguate（client_name / vendor_name → ambiguity error）と fanout（Document 系は no disambiguation）を明示
//   - Document 4 種の status は構造的不可のため schema 削除（D1, primary defense として handler reject 残置）
//   - 将来拡張フラグ（contingently unimplemented）は schema 残し description で警告（D4）
//   - find_projects.status は narrowing 必須（N05、API delegation 不可）を description で警告
func RegisterTools(s *Server) {
	s.MCPServer().AddTools(
		// --- Simple tools (id, name, limit) ---
		findClientsTool(s),
		findVendorsTool(s),
		findUsersTool(s),

		// --- Project tool (id, client_name, name, status, limit) ---
		findProjectsTool(s),

		// --- Client-document tools (fanout: id, project_id, client_name, project_name, limit) ---
		findEstimatesTool(s),
		findInvoicesTool(s),
		findOrdersTool(s),
		findDeliveriesTool(s),
		findReceiptsTool(s),

		// --- Vendor-document tool (id, vendor_name, project_name, status, limit) ---
		findPurchaseOrdersTool(s),

		// --- Payment tool (id, vendor_name, purchase_order_id, status, limit) ---
		findPaymentsTool(s),
	)
}

// readOnlyAnnotation returns ToolOptions marking a tool as read-only.
func readOnlyAnnotation() mcp.ToolOption {
	return mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    boolPtr(true),
		DestructiveHint: boolPtr(false),
	})
}

func boolPtr(b bool) *bool { return &b }

// --- Description helpers (N08) ---

// disambiguateNameDesc は name → ID 解決系プロパティの description を生成する。
// 重複ヒット時は ambiguity error + 候補列挙を返す挙動を明示。
func disambiguateNameDesc(target, entity string) string {
	return fmt.Sprintf("Resolve %s name (substring match) to filter %s. Returns an ambiguity error with up to 5 candidates if multiple %ss match; use id to disambiguate.", target, entity, target)
}

// fanoutNameDesc は Document 4 種の name 系プロパティ description を生成する。
// disambiguation を行わず全マッチ entity を fanout 検索することを明示。
func fanoutNameDesc(target, entity, narrowProp string) string {
	return fmt.Sprintf("Resolve %s name (substring match) and fanout-search across all matching %ss (no disambiguation). Use %s to narrow to a single document.", target, target, narrowProp)
}

// notYetSupportedDesc は contingently unimplemented なフラグの description を生成する。
func notYetSupportedDesc(field, scope string) string {
	return fmt.Sprintf("(NOT YET SUPPORTED) Will filter %s by %s in a future release. Currently returns an error.", scope, field)
}

func limitDesc() string {
	return "Max results to return (default: 50, max: 100)."
}

// getStringArg extracts a string argument from a CallToolRequest.
// Returns "" if the key is missing or not a string.
func getStringArg(req mcp.CallToolRequest, key string) string {
	args := req.GetArguments()
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// getIntArg extracts an integer argument from a CallToolRequest.
// JSON numbers arrive as float64, so this handles the conversion.
// Returns 0 if the key is missing or not a number.
func getIntArg(req mcp.CallToolRequest, key string) int {
	args := req.GetArguments()
	if args == nil {
		return 0
	}
	v, ok := args[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return int(i)
	default:
		return 0
	}
}

// marshalResult serializes a value to JSON and returns it as a text content result.
func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

// findResourceMap は各 find tool が触りうる resource 名を返す。
// CLI の findResourceMap と同期している必要がある。
var findResourceMap = map[string][]string{
	"client":         {"clients", "client_branches", "contacts"},
	"vendor":         {"vendors", "vendor_branches", "vendor_contacts"},
	"user":           {"users"},
	"project":        {"projects", "clients"},
	"estimate":       {"clients", "projects", "estimates"},
	"invoice":        {"invoices", "clients", "projects"},
	"order":          {"clients", "projects", "orders"},
	"delivery":       {"clients", "projects", "deliveries"},
	"receipt":        {"clients", "projects", "receipts"},
	"purchase_order": {"purchase_orders", "vendors", "projects"},
	"payment":        {"payments", "vendors"},
}

// findOutput は MCP find tool の統一レスポンス形式。
// CLI 側 cli.FindOutput と同等の構造（json タグも一致）。
type findOutput[T any] struct {
	Items []T          `json:"items"`
	Cache []cache.Info `json:"cache"`
}

// wrapAndMarshal は find 結果と該当 resource の cache info を同梱して JSON にする。
// items が nil の場合は空配列で表現する。SyncStore 未設定時 cache は空配列。
func wrapAndMarshal[T any](ctx context.Context, srv *Server, kind string, items []T) (*mcp.CallToolResult, error) {
	if items == nil {
		items = []T{}
	}
	out := findOutput[T]{Items: items, Cache: []cache.Info{}}
	if resources, ok := findResourceMap[kind]; ok && srv != nil && srv.SyncStore() != nil {
		out.Cache = cache.LoadInfos(ctx, srv.SyncStore(), srv.Profile(), resources)
	}
	return marshalResult(out)
}

// errorResult returns a tool error result from an error.
// RefreshInProgressError は構造化エラー（error_code, retry_after_seconds 含む）として返す。
func errorResult(err error) *mcp.CallToolResult {
	var rip *refresh.RefreshInProgressError
	if errors.As(err, &rip) {
		body := struct {
			Error             bool   `json:"error"`
			ErrorCode         string `json:"error_code"`
			Message           string `json:"message"`
			Resource          string `json:"resource"`
			Holder            string `json:"holder,omitempty"`
			ElapsedSeconds    int    `json:"elapsed_seconds"`
			RetryAfterSeconds int    `json:"retry_after_seconds"`
		}{
			Error:             true,
			ErrorCode:         "refresh_in_progress",
			Message:           rip.Error(),
			Resource:          rip.Resource,
			Holder:            rip.Holder,
			ElapsedSeconds:    rip.ElapsedSeconds,
			RetryAfterSeconds: rip.RetryAfterSeconds,
		}
		data, _ := json.Marshal(body)
		// MCP の IsError=true で構造化 JSON を text content として返す。
		return mcp.NewToolResultError(string(data))
	}
	return mcp.NewToolResultError(err.Error())
}

// parseRefreshArg は MCP 引数 refresh ("none"/"delta"/"full") を ReadOptions に変換する。
// "" / "none" / unknown は no-op (delta=false, full=false)。
// "delta" は Refresh=true。
// "full" は ForceRefresh=true（Refresh より優先）。
func parseRefreshArg(v string) repository.ReadOptions {
	switch v {
	case "delta":
		return repository.ReadOptions{Refresh: true}
	case "full":
		return repository.ReadOptions{ForceRefresh: true}
	}
	return repository.ReadOptions{}
}

func refreshDesc() string {
	return `Trigger a refresh before reading cache. "none" (default) reads cache as-is. "delta" fetches incremental changes since last sync. "full" re-fetches all entries and removes stale ones (use periodically). Concurrent refreshes return refresh_in_progress; clients should wait and retry.`
}

// refreshToolOption は各 find tool に refresh enum パラメータを追加するヘルパー。
func refreshToolOption() mcp.ToolOption {
	return mcp.WithString("refresh", mcp.Description(refreshDesc()), mcp.Enum("none", "delta", "full"))
}

// --- Tool definitions ---

func findClientsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_clients",
			mcp.WithDescription("Search BOARD clients by ID or name. Returns client entities with branches and contacts. Priority: id > name."),
			mcp.WithNumber("id", mcp.Description("Client ID for direct lookup (highest priority; ignores name).")),
			mcp.WithString("name", mcp.Description("Substring match on client name.")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindClient(ctx, find.FindClientQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "client", results)
		},
	}
}

func findVendorsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_vendors",
			mcp.WithDescription("Search BOARD vendors by ID or name. Returns vendor entities with branches and contacts. Priority: id > name."),
			mcp.WithNumber("id", mcp.Description("Vendor ID for direct lookup (highest priority; ignores name).")),
			mcp.WithString("name", mcp.Description("Substring match on vendor name.")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindVendor(ctx, find.FindVendorQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "vendor", results)
		},
	}
}

func findUsersTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_users",
			mcp.WithDescription("Search BOARD users by ID or name. Returns user entities. Priority: id > name."),
			mcp.WithNumber("id", mcp.Description("User ID for direct lookup (highest priority; ignores name).")),
			mcp.WithString("name", mcp.Description("Substring match on user name.")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindUser(ctx, find.FindUserQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "user", results)
		},
	}
}

func findProjectsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_projects",
			mcp.WithDescription("Search BOARD projects by ID, client name, or project name. Fields combine with AND (ID overrides others). Returns project entities with enriched client info. client_name resolves with ambiguity error on multiple matches. status/statuses/contract_status filtering requires narrowing (must combine with id/client_name/name). status, statuses, contract_status are mutually exclusive."),
			mcp.WithNumber("id", mcp.Description("Project ID for direct lookup (highest priority; ignores other filters).")),
			mcp.WithString("client_name", mcp.Description(disambiguateNameDesc("client", "project"))),
			mcp.WithString("name", mcp.Description("Substring match on project name.")),
			mcp.WithString("status", mcp.Description("Filter by project status (e.g. 受注, 完了). MUST be combined with id/client_name/name — status-only query is rejected (API delegation not possible, narrowing required). Mutually exclusive with statuses / contract_status.")),
			mcp.WithArray("statuses", mcp.WithStringItems(), mcp.Description("Filter by multiple project statuses (OR). Mutually exclusive with status / contract_status. Same narrowing rules apply. Max 10 items.")),
			mcp.WithString("contract_status", mcp.Description("Contract status alias. Valid values: active (in-progress: 未着手/着手中/納品済), ended (検収済), prospect (見積中*), all. Mutually exclusive with status / statuses. Same narrowing rules apply. See docs/usage/maintenance-contract-search.md.")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			svc := srv.FindService()
			var clientID int
			if cn := getStringArg(req, "client_name"); cn != "" {
				id, err := svc.ResolveClientByName(ctx, cn, repository.ReadOptions{})
				if err != nil {
					return errorResult(err), nil
				}
				clientID = id
			}
			results, err := svc.FindProject(ctx, find.FindProjectQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ClientID:       clientID,
				Name:           getStringArg(req, "name"),
				Status:         getStringArg(req, "status"),
				Statuses:       getStringArrayArg(req, "statuses"),
				ContractStatus: getStringArg(req, "contract_status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "project", results)
		},
	}
}

func findEstimatesTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_estimates",
			mcp.WithDescription("Search BOARD estimate documents by ID, project ID, client name, or project name. Returns estimate entities (no Status field on entity, hence no status filter). client_name/project_name perform fanout search across all matching entities (no disambiguation)."),
			mcp.WithNumber("id", mcp.Description("Estimate document ID for direct lookup (highest priority).")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its estimate (use to narrow to a single document).")),
			mcp.WithString("client_name", mcp.Description(fanoutNameDesc("client", "estimate", "project_id"))),
			mcp.WithString("project_name", mcp.Description(fanoutNameDesc("project", "estimate", "project_id"))),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// N08 D1: status は schema 削除済だが primary defense として handler reject 残置。
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindEstimate(ctx, find.FindEstimateQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "estimate", results)
		},
	}
}

func findInvoicesTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_invoices",
			mcp.WithDescription("Search BOARD invoices by ID, client name, or status. Fields combine with AND (ID overrides others). Returns invoice entities. client_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Invoice ID for direct lookup (highest priority).")),
			mcp.WithString("client_name", mcp.Description(disambiguateNameDesc("client", "invoice"))),
			mcp.WithString("project_name", mcp.Description(notYetSupportedDesc("project name", "invoices"))),
			mcp.WithString("status", mcp.Description("Filter by invoice status (single value; API delegated, no narrowing required).")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "project_name") != "" {
				return errorResult(fmt.Errorf("project_name is not yet supported for invoices (tracked for future enhancement)")), nil
			}
			svc := srv.FindService()
			var clientID int
			if cn := getStringArg(req, "client_name"); cn != "" {
				id, err := svc.ResolveClientByName(ctx, cn, repository.ReadOptions{})
				if err != nil {
					return errorResult(err), nil
				}
				clientID = id
			}
			results, err := svc.FindInvoice(ctx, find.FindInvoiceQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ClientID:       clientID,
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "invoice", results)
		},
	}
}

func findOrdersTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_orders",
			mcp.WithDescription("Search BOARD order documents by ID, project ID, client name, or project name. Returns order entities (no Status field on entity, hence no status filter). client_name/project_name perform fanout search across all matching entities (no disambiguation)."),
			mcp.WithNumber("id", mcp.Description("Order document ID for direct lookup (highest priority).")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its order (use to narrow to a single document).")),
			mcp.WithString("client_name", mcp.Description(fanoutNameDesc("client", "order", "project_id"))),
			mcp.WithString("project_name", mcp.Description(fanoutNameDesc("project", "order", "project_id"))),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindOrder(ctx, find.FindOrderQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "order", results)
		},
	}
}

func findDeliveriesTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_deliveries",
			mcp.WithDescription("Search BOARD delivery documents by ID, project ID, client name, or project name. Returns delivery entities (no Status field on entity, hence no status filter). client_name/project_name perform fanout search across all matching entities (no disambiguation)."),
			mcp.WithNumber("id", mcp.Description("Delivery document ID for direct lookup (highest priority).")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its deliveries (use to narrow to a single project).")),
			mcp.WithString("client_name", mcp.Description(fanoutNameDesc("client", "delivery", "project_id"))),
			mcp.WithString("project_name", mcp.Description(fanoutNameDesc("project", "delivery", "project_id"))),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindDelivery(ctx, find.FindDeliveryQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "delivery", results)
		},
	}
}

func findReceiptsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_receipts",
			mcp.WithDescription("Search BOARD receipt documents by ID, project ID, client name, or project name. Returns receipt entities (no Status field on entity, hence no status filter). client_name/project_name perform fanout search across all matching entities (no disambiguation)."),
			mcp.WithNumber("id", mcp.Description("Receipt document ID for direct lookup (highest priority).")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its receipts (use to narrow to a single project).")),
			mcp.WithString("client_name", mcp.Description(fanoutNameDesc("client", "receipt", "project_id"))),
			mcp.WithString("project_name", mcp.Description(fanoutNameDesc("project", "receipt", "project_id"))),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindReceipt(ctx, find.FindReceiptQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "receipt", results)
		},
	}
}

func findPurchaseOrdersTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_purchase_orders",
			mcp.WithDescription("Search BOARD purchase orders by ID, vendor name, or status. Fields combine with AND (ID overrides others). Returns purchase order entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Purchase order ID for direct lookup (highest priority).")),
			mcp.WithString("vendor_name", mcp.Description(disambiguateNameDesc("vendor", "purchase order"))),
			mcp.WithString("project_name", mcp.Description(notYetSupportedDesc("project name", "purchase orders"))),
			mcp.WithString("status", mcp.Description("Filter by purchase order status (single value; API delegated, no narrowing required).")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "project_name") != "" {
				return errorResult(fmt.Errorf("project_name is not yet supported for purchase orders (tracked for future enhancement)")), nil
			}
			svc := srv.FindService()
			var vendorID int
			if vn := getStringArg(req, "vendor_name"); vn != "" {
				id, err := svc.ResolveVendorByName(ctx, vn, repository.ReadOptions{})
				if err != nil {
					return errorResult(err), nil
				}
				vendorID = id
			}
			results, err := svc.FindPurchaseOrder(ctx, find.FindPurchaseOrderQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				VendorID:       vendorID,
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "purchase_order", results)
		},
	}
}

func findPaymentsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_payments",
			mcp.WithDescription("Search BOARD payments by ID, vendor name, or status. Fields combine with AND (ID overrides others). Returns payment entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). purchase_order_id is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Payment ID for direct lookup (highest priority).")),
			mcp.WithString("vendor_name", mcp.Description(disambiguateNameDesc("vendor", "payment"))),
			mcp.WithNumber("purchase_order_id", mcp.Description(notYetSupportedDesc("purchase order ID", "payments"))),
			mcp.WithString("status", mcp.Description("Filter by payment status (single value; API delegated, no narrowing required).")),
			refreshToolOption(),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getIntArg(req, "purchase_order_id") != 0 {
				return errorResult(fmt.Errorf("purchase_order_id is not yet supported (tracked for future enhancement)")), nil
			}
			svc := srv.FindService()
			var vendorID int
			if vn := getStringArg(req, "vendor_name"); vn != "" {
				id, err := svc.ResolveVendorByName(ctx, vn, repository.ReadOptions{})
				if err != nil {
					return errorResult(err), nil
				}
				vendorID = id
			}
			results, err := svc.FindPayment(ctx, find.FindPaymentQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit"), Opts: parseRefreshArg(getStringArg(req, "refresh"))},
				ID:             getIntArg(req, "id"),
				VendorID:       vendorID,
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return wrapAndMarshal(ctx, srv, "payment", results)
		},
	}
}

// getStringArrayArg extracts a []string argument from a CallToolRequest.
// JSON arrays arrive as []any; non-string elements are silently skipped.
// Returns nil if the key is missing, the args map is nil, or the value is not a []any.
func getStringArrayArg(req mcp.CallToolRequest, key string) []string {
	args := req.GetArguments()
	if args == nil {
		return nil
	}
	v, ok := args[key]
	if !ok {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, elem := range raw {
		if s, ok := elem.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
