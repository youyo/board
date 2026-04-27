package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
		// --- Simple tools (id, name, text, limit) ---
		findClientsTool(s),
		findVendorsTool(s),
		findUsersTool(s),

		// --- Project tool (id, client_name, name, text, status, limit) ---
		findProjectsTool(s),

		// --- Client-document tools (fanout: id, project_id, client_name, project_name, limit) ---
		findEstimatesTool(s),
		findInvoicesTool(s),
		findOrdersTool(s),
		findDeliveriesTool(s),
		findReceiptsTool(s),

		// --- Vendor-document tool (id, vendor_name, project_name, text, status, limit) ---
		findPurchaseOrdersTool(s),

		// --- Payment tool (id, vendor_name, purchase_order_id, text, status, limit) ---
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

// errorResult returns a tool error result from an error.
func errorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// --- Tool definitions ---

func findClientsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_clients",
			mcp.WithDescription("Search BOARD clients by ID, name, or free text. Returns client entities with branches and contacts. Priority: id > name > text."),
			mcp.WithNumber("id", mcp.Description("Client ID for direct lookup (highest priority; ignores name/text).")),
			mcp.WithString("name", mcp.Description("Substring match on client name (ignores text).")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority).")),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindClient(ctx, find.FindClientQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
				Text:           getStringArg(req, "text"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findVendorsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_vendors",
			mcp.WithDescription("Search BOARD vendors by ID, name, or free text. Returns vendor entities with branches and contacts. Priority: id > name > text."),
			mcp.WithNumber("id", mcp.Description("Vendor ID for direct lookup (highest priority; ignores name/text).")),
			mcp.WithString("name", mcp.Description("Substring match on vendor name (ignores text).")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority).")),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindVendor(ctx, find.FindVendorQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
				Text:           getStringArg(req, "text"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findUsersTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_users",
			mcp.WithDescription("Search BOARD users by ID, name, or free text. Returns user entities. Priority: id > name > text."),
			mcp.WithNumber("id", mcp.Description("User ID for direct lookup (highest priority; ignores name/text).")),
			mcp.WithString("name", mcp.Description("Substring match on user name (ignores text).")),
			mcp.WithString("text", mcp.Description("Free-text search across name, email (lowest priority).")),
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := srv.FindService().FindUser(ctx, find.FindUserQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				Name:           getStringArg(req, "name"),
				Text:           getStringArg(req, "text"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findProjectsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_projects",
			mcp.WithDescription("Search BOARD projects by ID, client name, project name, or free text. Returns project entities with enriched client info. client_name resolves with ambiguity error on multiple matches. status/statuses/contract_status filtering requires narrowing (must combine with id/client_name/name/text). status, statuses, contract_status are mutually exclusive."),
			mcp.WithNumber("id", mcp.Description("Project ID for direct lookup (highest priority; ignores other filters).")),
			mcp.WithString("client_name", mcp.Description(disambiguateNameDesc("client", "project"))),
			mcp.WithString("name", mcp.Description("Substring match on project name.")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority).")),
			mcp.WithString("status", mcp.Description("Filter by project status (e.g. 受注, 完了). MUST be combined with id/client_name/name/text — status-only query is rejected (API delegation not possible, narrowing required per N05). Mutually exclusive with statuses / contract_status.")),
			mcp.WithArray("statuses", mcp.WithStringItems(), mcp.Description("Filter by multiple project statuses (OR). Mutually exclusive with status / contract_status. Same narrowing rules apply. Max 10 items.")),
			mcp.WithString("contract_status", mcp.Description("Contract status alias. Valid values: active (in-progress: 未着手/着手中/納品済), ended (検収済), prospect (見積中*), all. Mutually exclusive with status / statuses. Same narrowing rules apply. See docs/usage/maintenance-contract-search.md.")),
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
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ClientID:       clientID,
				Name:           getStringArg(req, "name"),
				Text:           getStringArg(req, "text"),
				Status:         getStringArg(req, "status"),
				Statuses:       getStringArrayArg(req, "statuses"),
				ContractStatus: getStringArg(req, "contract_status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
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
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// N08 D1: status は schema 削除済だが primary defense として handler reject 残置。
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindEstimate(ctx, find.FindEstimateQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findInvoicesTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_invoices",
			mcp.WithDescription("Search BOARD invoices by ID, client name, project name, or free text. Returns invoice entities. client_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Invoice ID for direct lookup (highest priority).")),
			mcp.WithString("client_name", mcp.Description(disambiguateNameDesc("client", "invoice"))),
			mcp.WithString("project_name", mcp.Description(notYetSupportedDesc("project name", "invoices"))),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority).")),
			mcp.WithString("status", mcp.Description("Filter by invoice status (single value; API delegated, no narrowing required).")),
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
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ClientID:       clientID,
				Text:           getStringArg(req, "text"),
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
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
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindOrder(ctx, find.FindOrderQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
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
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindDelivery(ctx, find.FindDeliveryQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
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
			mcp.WithNumber("limit", mcp.Description(limitDesc())),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if getStringArg(req, "status") != "" {
				return errorResult(fmt.Errorf("status filtering is not supported for documents (no Status field on entity)")), nil
			}
			results, err := srv.FindService().FindReceipt(ctx, find.FindReceiptQuery{
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				ProjectID:      getIntArg(req, "project_id"),
				ClientName:     getStringArg(req, "client_name"),
				ProjectName:    getStringArg(req, "project_name"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findPurchaseOrdersTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_purchase_orders",
			mcp.WithDescription("Search BOARD purchase orders by ID, vendor name, project name, or free text. Returns purchase order entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Purchase order ID for direct lookup (highest priority).")),
			mcp.WithString("vendor_name", mcp.Description(disambiguateNameDesc("vendor", "purchase order"))),
			mcp.WithString("project_name", mcp.Description(notYetSupportedDesc("project name", "purchase orders"))),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority).")),
			mcp.WithString("status", mcp.Description("Filter by purchase order status (single value; API delegated, no narrowing required).")),
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
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				VendorID:       vendorID,
				Text:           getStringArg(req, "text"),
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
		},
	}
}

func findPaymentsTool(srv *Server) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_payments",
			mcp.WithDescription("Search BOARD payments by ID, vendor name, or free text. Returns payment entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). purchase_order_id is not yet supported."),
			mcp.WithNumber("id", mcp.Description("Payment ID for direct lookup (highest priority).")),
			mcp.WithString("vendor_name", mcp.Description(disambiguateNameDesc("vendor", "payment"))),
			mcp.WithNumber("purchase_order_id", mcp.Description(notYetSupportedDesc("purchase order ID", "payments"))),
			mcp.WithString("text", mcp.Description("Free-text search across memo (lowest priority).")),
			mcp.WithString("status", mcp.Description("Filter by payment status (single value; API delegated, no narrowing required).")),
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
				FindCommonOpts: find.FindCommonOpts{Limit: getIntArg(req, "limit")},
				ID:             getIntArg(req, "id"),
				VendorID:       vendorID,
				Text:           getStringArg(req, "text"),
				Status:         getStringArg(req, "status"),
			})
			if err != nil {
				return errorResult(err), nil
			}
			return marshalResult(results)
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
