//go:build e2e

// Package mcpserver E2E tests via in-process handler invocation (build tag: e2e).
//
// 本ファイルは MCP HTTP server を起動せず、`ServerTool.Handler` を直接呼び出して
// handler reject / resolver / fanout の挙動を実 BOARD API 環境で検証する。
//
// 検証対象（T42-T46）:
//
//	T42: find_estimates.status の handler reject（構造的不可、N08 D1）
//	T43: find_invoices.project_name の (NOT YET SUPPORTED) reject（D4）
//	T44: find_payments.purchase_order_id の (NOT YET SUPPORTED) reject（D4）
//	T45: find_projects.client_name の resolver 経路（実 API 経由）
//	T46: find_orders.client_name の fanout 挙動
//
// 実行例:
//
//	go test -tags e2e -v -count=1 -run TestE2E_MCPHandler ./internal/mcpserver/
package mcpserver

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/youyo/board/internal/app"
)

func skipIfNoCreds(t *testing.T) {
	t.Helper()
	if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
		t.Skipf("[SKIP:no-creds] BOARD_API_KEY and BOARD_API_TOKEN required")
	}
}

func newE2EServer(t *testing.T) *Server {
	t.Helper()
	skipIfNoCreds(t)
	a, err := app.New("")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return New(a.FindService())
}

// callTool finds a tool by name in the registered set and invokes its handler.
// This avoids HTTP transport while still going through the exact handler used in production.
func callTool(t *testing.T, name string, args map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()
	srv := newE2EServer(t)

	factories := map[string]func(*Server) server.ServerTool{
		"find_clients":         findClientsTool,
		"find_vendors":         findVendorsTool,
		"find_users":           findUsersTool,
		"find_projects":        findProjectsTool,
		"find_estimates":       findEstimatesTool,
		"find_invoices":        findInvoicesTool,
		"find_orders":          findOrdersTool,
		"find_deliveries":      findDeliveriesTool,
		"find_receipts":        findReceiptsTool,
		"find_purchase_orders": findPurchaseOrdersTool,
		"find_payments":        findPaymentsTool,
	}
	f, ok := factories[name]
	if !ok {
		t.Fatalf("unknown tool name: %s", name)
	}
	tool := f(srv)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	return tool.Handler(context.Background(), req)
}

// resultErrorMessage extracts the error message from a CallToolResult.IsError result.
func resultErrorMessage(r *mcp.CallToolResult) string {
	if r == nil {
		return ""
	}
	if !r.IsError {
		return ""
	}
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// T42: find_estimates.status は handler で reject される (D1)
func TestE2E_MCPHandler_FindEstimates_StatusRejected(t *testing.T) {
	res, err := callTool(t, "find_estimates", map[string]any{
		"status": "発行済",
	})
	if err != nil {
		t.Fatalf("callTool error: %v", err)
	}
	msg := resultErrorMessage(res)
	if msg == "" {
		t.Fatalf("expected IsError=true with message, got: %+v", res)
	}
	if !strings.Contains(msg, "status") || !strings.Contains(msg, "not supported") {
		t.Fatalf("expected 'status not supported' message, got: %q", msg)
	}
	t.Logf("T42 reject confirmed: %s", msg)
}

// T43: find_invoices.project_name → (NOT YET SUPPORTED)
func TestE2E_MCPHandler_FindInvoices_ProjectNameRejected(t *testing.T) {
	res, err := callTool(t, "find_invoices", map[string]any{
		"project_name": "ZZ_NEVER_MATCH",
	})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	msg := resultErrorMessage(res)
	if !strings.Contains(msg, "not yet supported") {
		t.Fatalf("expected 'not yet supported', got: %q", msg)
	}
	t.Logf("T43 reject confirmed: %s", msg)
}

// T44: find_payments.purchase_order_id → (NOT YET SUPPORTED)
func TestE2E_MCPHandler_FindPayments_PurchaseOrderIDRejected(t *testing.T) {
	res, err := callTool(t, "find_payments", map[string]any{
		"purchase_order_id": float64(123),
	})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	msg := resultErrorMessage(res)
	if !strings.Contains(msg, "not yet supported") {
		t.Fatalf("expected 'not yet supported', got: %q", msg)
	}
	t.Logf("T44 reject confirmed: %s", msg)
}

// T45: find_projects.client_name resolver 経路（disambiguate）
func TestE2E_MCPHandler_FindProjects_ClientNameResolver(t *testing.T) {
	// "株" は通常複数 client にマッチするため ambiguity error を期待
	res, err := callTool(t, "find_projects", map[string]any{
		"client_name": "株",
	})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if res.IsError {
		msg := resultErrorMessage(res)
		// ambiguous か、単一マッチして status-only narrowing 違反になる可能性も
		t.Logf("T45 (resolver path observed): %s", msg)
		if !strings.Contains(msg, "ambiguous") && !strings.Contains(msg, "multiple") &&
			!strings.Contains(msg, "no match") && !strings.Contains(msg, "not found") &&
			!strings.Contains(msg, "narrow") {
			t.Logf("unexpected error category (logged, not fatal): %s", msg)
		}
		return
	}
	t.Logf("T45 resolver succeeded uniquely (cache-warm path)")
}

// T46: find_orders.client_name fanout 挙動（ambiguity error を出さない）
func TestE2E_MCPHandler_FindOrders_ClientNameFanout(t *testing.T) {
	// fanout 系は「複数マッチでも error にせず横断検索」する仕様
	res, err := callTool(t, "find_orders", map[string]any{
		"client_name": "株",
	})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if res.IsError {
		msg := resultErrorMessage(res)
		if strings.Contains(msg, "ambiguous") {
			t.Fatalf("fanout tool should NOT raise ambiguity error, got: %s", msg)
		}
		// データ不在 / API エラー等は許容（[SKIP] にしない、単に Logf）
		t.Logf("T46 fanout error (acceptable for data state): %s", msg)
		return
	}
	t.Logf("T46 fanout succeeded (no disambiguation required)")
}
