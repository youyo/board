package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers all 12 MCP tool definitions with placeholder handlers.
// Real handlers are wired in M37 (core + documents) and M38 (vendor + master).
func RegisterTools(s *Server) {
	s.MCPServer().AddTools(
		// --- Simple tools (id, name, text, limit) ---
		findClientsTool(),
		findVendorsTool(),
		findUsersTool(),
		findGroupsTool(),

		// --- Project tool (id, client_name, name, text, status, limit) ---
		findProjectsTool(),

		// --- Client-document tools (id, client_name, project_name, text, status, limit) ---
		findEstimatesTool(),
		findInvoicesTool(),
		findOrdersTool(),
		findDeliveriesTool(),
		findReceiptsTool(),

		// --- Vendor-document tool (id, vendor_name, project_name, text, status, limit) ---
		findPurchaseOrdersTool(),

		// --- Payment tool (id, vendor_name, purchase_order_id, text, status, limit) ---
		findPaymentsTool(),
	)
}

// notImplementedHandler is a placeholder handler for tools not yet implemented.
func notImplementedHandler(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(fmt.Sprintf("tool %q is not yet implemented", request.Params.Name)), nil
}

// newEmptyCallToolRequest creates a CallToolRequest for testing purposes.
func newEmptyCallToolRequest(name string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: name,
		},
	}
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

func findClientsTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_clients",
			mcp.WithDescription("Search for clients by ID, name, or free text. Returns client details with branches and contacts."),
			mcp.WithNumber("id", mcp.Description("Client ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on client name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findVendorsTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_vendors",
			mcp.WithDescription("Search for vendors by ID, name, or free text. Returns vendor details with branches and contacts."),
			mcp.WithNumber("id", mcp.Description("Vendor ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on vendor name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, code, memo (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findUsersTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_users",
			mcp.WithDescription("Search for users by ID, name, or free text. Returns user details."),
			mcp.WithNumber("id", mcp.Description("User ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on user name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, email (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findGroupsTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_groups",
			mcp.WithDescription("Search for groups by ID, name, or free text. Returns group details."),
			mcp.WithNumber("id", mcp.Description("Group ID for direct lookup (highest priority, ignores name/text)")),
			mcp.WithString("name", mcp.Description("Substring match on group name (ignores text)")),
			mcp.WithString("text", mcp.Description("Free-text search across name, memo (lowest priority)")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findProjectsTool() server.ServerTool {
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
		Handler: notImplementedHandler,
	}
}

func findEstimatesTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_estimates",
			mcp.WithDescription("Search for estimates by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Estimate ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter estimates by client")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter estimates by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by estimate status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findInvoicesTool() server.ServerTool {
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
		Handler: notImplementedHandler,
	}
}

func findOrdersTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_orders",
			mcp.WithDescription("Search for orders by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Order ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter orders by client")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter orders by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by order status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findDeliveriesTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_deliveries",
			mcp.WithDescription("Search for deliveries by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Delivery ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter deliveries by client")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter deliveries by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by delivery status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findReceiptsTool() server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("find_receipts",
			mcp.WithDescription("Search for receipts by ID, client name, project name, or free text. Supports status filtering."),
			mcp.WithNumber("id", mcp.Description("Receipt ID for direct lookup (highest priority)")),
			mcp.WithString("client_name", mcp.Description("Resolve client name to filter receipts by client")),
			mcp.WithString("project_name", mcp.Description("Resolve project name to filter receipts by project")),
			mcp.WithString("text", mcp.Description("Free-text search across title, memo (lowest priority)")),
			mcp.WithString("status", mcp.Description("Filter by receipt status")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default: unlimited)")),
			readOnlyAnnotation(),
		),
		Handler: notImplementedHandler,
	}
}

func findPurchaseOrdersTool() server.ServerTool {
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
		Handler: notImplementedHandler,
	}
}

func findPaymentsTool() server.ServerTool {
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
		Handler: notImplementedHandler,
	}
}
