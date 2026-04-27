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
func RegisterTools(s *Server) {
	s.MCPServer().AddTools(
		// --- Simple tools (id, name, text, limit) ---
		findClientsTool(s),
		findVendorsTool(s),
		findUsersTool(s),

		// --- Project tool (id, client_name, name, text, status, limit) ---
		findProjectsTool(s),

		// --- Client-document tools (id, client_name, project_name, text, status, limit) ---
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
			mcp.WithDescription("Search for clients by ID, name, or free text. Returns client details with branches and contacts."),
			mcp.WithNumber("id", mcp.Description("Client ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on client name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for vendors by ID, name, or free text. Returns vendor details with branches and contacts."),
			mcp.WithNumber("id", mcp.Description("Vendor ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on vendor name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for users by ID, name, or free text. Returns user details."),
			mcp.WithNumber("id", mcp.Description("User ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on user name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, email (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for projects by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Project ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter projects by client")),
			mcp.WithString("name", mcp.Description("Substring match on project name")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by project status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for estimates by ID, project ID, client name, or project name. Supports status post-filtering."),
			mcp.WithNumber("id", mcp.Description("Estimate document ID for direct lookup (highest priority)")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its estimate")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to find estimates via projects")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to find estimates")),
			mcp.WithString("status", mcp.Description("Post-filter by estimate status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			mcp.WithDescription("Search for invoices by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Invoice ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter invoices by client")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter invoices by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by invoice status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for orders by ID, project ID, client name, or project name. Supports status post-filtering."),
			mcp.WithNumber("id", mcp.Description("Order document ID for direct lookup (highest priority)")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its order")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to find orders via projects")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to find orders")),
			mcp.WithString("status", mcp.Description("Post-filter by order status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for deliveries by ID, project ID, client name, or project name. Supports status post-filtering."),
			mcp.WithNumber("id", mcp.Description("Delivery document ID for direct lookup (highest priority)")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its delivery")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to find deliveries via projects")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to find deliveries")),
			mcp.WithString("status", mcp.Description("Post-filter by delivery status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for receipts by ID, project ID, client name, or project name. Supports status post-filtering."),
			mcp.WithNumber("id", mcp.Description("Receipt document ID for direct lookup (highest priority)")),
			mcp.WithNumber("project_id", mcp.Description("Project ID to find its receipt")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to find receipts via projects")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to find receipts")),
			mcp.WithString("status", mcp.Description("Post-filter by receipt status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for purchase orders by ID, vendor name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Purchase order ID for direct lookup (highest priority)")),
			mcp.WithString("vendor_name", mcp.Description("Resolve vendor name to filter purchase orders by vendor")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter purchase orders by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by purchase order status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
			mcp.WithDescription("Search for payments by ID, vendor name, purchase order ID, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Payment ID for direct lookup (highest priority)")),
			mcp.WithString("vendor_name", mcp.Description("Resolve vendor name to filter payments by vendor")),
			mcp.WithNumber("purchase_order_id", mcp.Description("Filter payments by purchase order ID")),
			mcp.WithString("text", mcp.Description("Free-text search across memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by payment status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
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
