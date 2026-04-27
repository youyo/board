package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRegisterTools_Count(t *testing.T) {
	s := New(nil)

	tools := s.MCPServer().ListTools()
	if len(tools) != 11 {
		t.Fatalf("expected 11 tools, got %d", len(tools))
	}
}

func TestRegisterTools_Names(t *testing.T) {
	s := New(nil)

	expected := []string{
		"find_clients",
		"find_projects",
		"find_estimates",
		"find_invoices",
		"find_orders",
		"find_deliveries",
		"find_receipts",
		"find_vendors",
		"find_purchase_orders",
		"find_payments",
		"find_users",
	}

	tools := s.MCPServer().ListTools()
	nameSet := make(map[string]bool)
	for name := range tools {
		nameSet[name] = true
	}

	for _, name := range expected {
		if !nameSet[name] {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestRegisterTools_Descriptions(t *testing.T) {
	s := New(nil)

	tools := s.MCPServer().ListTools()
	for name, tool := range tools {
		if tool.Tool.Description == "" {
			t.Errorf("tool %q has empty description", name)
		}
	}
}

func TestRegisterTools_InputSchemaProperties(t *testing.T) {
	s := New(nil)

	tools := s.MCPServer().ListTools()

	// Simple tools (clients, vendors, users, groups) should have: id, name, text, limit
	simpleCases := map[string][]string{
		"find_clients": {"id", "name", "text", "limit"},
		"find_vendors": {"id", "name", "text", "limit"},
		"find_users":   {"id", "name", "text", "limit"},
	}
	for toolName, expectedProps := range simpleCases {
		tool, ok := tools[toolName]
		if !ok {
			t.Errorf("tool %q not found", toolName)
			continue
		}
		props := tool.Tool.InputSchema.Properties
		for _, prop := range expectedProps {
			if _, exists := props[prop]; !exists {
				t.Errorf("tool %q missing property %q", toolName, prop)
			}
		}
	}

	// Project tool should have: id, client_name, name, text, status, limit
	projectProps := []string{"id", "client_name", "name", "text", "status", "limit"}
	if tool, ok := tools["find_projects"]; ok {
		for _, prop := range projectProps {
			if _, exists := tool.Tool.InputSchema.Properties[prop]; !exists {
				t.Errorf("tool find_projects missing property %q", prop)
			}
		}
	}

	// Invoice tool should have: id, client_name, project_name, text, status, limit
	invoiceProps := []string{"id", "client_name", "project_name", "text", "status", "limit"}
	for _, prop := range invoiceProps {
		tool, ok := tools["find_invoices"]
		if !ok {
			t.Error("tool \"find_invoices\" not found")
			break
		}
		if _, exists := tool.Tool.InputSchema.Properties[prop]; !exists {
			t.Errorf("tool %q missing property %q", "find_invoices", prop)
		}
	}

	// Estimate/Order/Delivery/Receipt tools use project_id instead of text.
	// status is structurally not supported (no Status field on Document entities) and removed from schema (N08).
	docTools := []string{"find_estimates", "find_orders", "find_deliveries", "find_receipts"}
	docProps := []string{"id", "project_id", "client_name", "project_name", "limit"}
	for _, toolName := range docTools {
		tool, ok := tools[toolName]
		if !ok {
			t.Errorf("tool %q not found", toolName)
			continue
		}
		for _, prop := range docProps {
			if _, exists := tool.Tool.InputSchema.Properties[prop]; !exists {
				t.Errorf("tool %q missing property %q", toolName, prop)
			}
		}
		// status must NOT be in schema for Document tools (N08 D1)
		if _, exists := tool.Tool.InputSchema.Properties["status"]; exists {
			t.Errorf("tool %q must NOT have 'status' property (N08: structurally unsupported)", toolName)
		}
	}

	// Vendor-doc: find_purchase_orders
	vendorDocProps := []string{"id", "vendor_name", "project_name", "text", "status", "limit"}
	if tool, ok := tools["find_purchase_orders"]; ok {
		for _, prop := range vendorDocProps {
			if _, exists := tool.Tool.InputSchema.Properties[prop]; !exists {
				t.Errorf("tool find_purchase_orders missing property %q", prop)
			}
		}
	}

	// Payment: find_payments
	paymentProps := []string{"id", "vendor_name", "purchase_order_id", "text", "status", "limit"}
	if tool, ok := tools["find_payments"]; ok {
		for _, prop := range paymentProps {
			if _, exists := tool.Tool.InputSchema.Properties[prop]; !exists {
				t.Errorf("tool find_payments missing property %q", prop)
			}
		}
	}
}

// N08: disambiguate を行うリゾルバ系 tool description が曖昧性挙動に言及していること。
func TestRegisterTools_DescriptionMentionsDisambiguation(t *testing.T) {
	s := New(nil)
	tools := s.MCPServer().ListTools()

	disambiguateTools := []string{"find_projects", "find_invoices", "find_purchase_orders", "find_payments"}
	for _, name := range disambiguateTools {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %q not found", name)
			continue
		}
		desc := tool.Tool.Description
		if !containsAny(desc, "ambiguity", "ambiguous", "disambiguate") {
			t.Errorf("tool %q description must mention disambiguation behavior, got: %q", name, desc)
		}
	}
}

// N08: Document 4 種は fanout 検索（disambiguate なし）であることを description で明示。
func TestRegisterTools_DescriptionMentionsFanout(t *testing.T) {
	s := New(nil)
	tools := s.MCPServer().ListTools()

	fanoutTools := []string{"find_estimates", "find_orders", "find_deliveries", "find_receipts"}
	for _, name := range fanoutTools {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %q not found", name)
			continue
		}
		desc := tool.Tool.Description
		if !containsAny(desc, "fanout", "fan-out") {
			t.Errorf("tool %q description must mention fanout behavior, got: %q", name, desc)
		}
	}
}

// N08: 将来拡張フラグ（contingently unimplemented）の property description で警告。
func TestRegisterTools_DescriptionMentionsNotYetSupported(t *testing.T) {
	s := New(nil)
	tools := s.MCPServer().ListTools()

	cases := []struct {
		toolName string
		propName string
	}{
		{"find_invoices", "project_name"},
		{"find_purchase_orders", "project_name"},
		{"find_payments", "purchase_order_id"},
	}
	for _, c := range cases {
		tool, ok := tools[c.toolName]
		if !ok {
			t.Errorf("tool %q not found", c.toolName)
			continue
		}
		propAny, ok := tool.Tool.InputSchema.Properties[c.propName]
		if !ok {
			t.Errorf("tool %q missing property %q", c.toolName, c.propName)
			continue
		}
		propMap, ok := propAny.(map[string]any)
		if !ok {
			t.Errorf("tool %q property %q is not a map: %T", c.toolName, c.propName, propAny)
			continue
		}
		desc, _ := propMap["description"].(string)
		if !containsAny(desc, "NOT YET SUPPORTED", "not yet supported") {
			t.Errorf("tool %q.%q description must mention 'not yet supported', got: %q", c.toolName, c.propName, desc)
		}
	}
}

// N08: find_projects の status は narrowing 必須（N05 確立、API delegation 不可）。
// description で narrowing 要件を明示し LLM の status-only クエリを抑止する。
func TestRegisterTools_FindProjectsStatusMentionsNarrowing(t *testing.T) {
	s := New(nil)
	tools := s.MCPServer().ListTools()

	tool, ok := tools["find_projects"]
	if !ok {
		t.Fatal("tool find_projects not found")
	}
	propAny, ok := tool.Tool.InputSchema.Properties["status"]
	if !ok {
		t.Fatal("find_projects missing 'status' property")
	}
	propMap, ok := propAny.(map[string]any)
	if !ok {
		t.Fatalf("find_projects.status is not a map: %T", propAny)
	}
	desc, _ := propMap["description"].(string)
	if !containsAny(desc, "narrow", "status-only", "must be combined") {
		t.Errorf("find_projects.status description must mention narrowing requirement, got: %q", desc)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func TestRegisterTools_NoRequiredFields(t *testing.T) {
	s := New(nil)

	tools := s.MCPServer().ListTools()
	for name, tool := range tools {
		if len(tool.Tool.InputSchema.Required) > 0 {
			t.Errorf("tool %q should have no required fields, got %v", name, tool.Tool.InputSchema.Required)
		}
	}
}

func TestRegisterTools_ReadOnlyAnnotation(t *testing.T) {
	s := New(nil)

	tools := s.MCPServer().ListTools()
	for name, tool := range tools {
		ann := tool.Tool.Annotations
		if ann.ReadOnlyHint == nil || !*ann.ReadOnlyHint {
			t.Errorf("tool %q should have ReadOnlyHint=true", name)
		}
		if ann.DestructiveHint == nil || *ann.DestructiveHint {
			t.Errorf("tool %q should have DestructiveHint=false", name)
		}
	}
}

func TestGetStringArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want string
	}{
		{"present", map[string]any{"name": "abc"}, "name", "abc"},
		{"missing", map[string]any{}, "name", ""},
		{"wrong_type", map[string]any{"name": 123}, "name", ""},
		{"nil_args", nil, "name", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			}
			if got := getStringArg(req, tt.key); got != tt.want {
				t.Errorf("getStringArg() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetIntArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want int
	}{
		{"float64", map[string]any{"id": float64(42)}, "id", 42},
		{"int", map[string]any{"id": 42}, "id", 42},
		{"json_number", map[string]any{"id": json.Number("99")}, "id", 99},
		{"missing", map[string]any{}, "id", 0},
		{"wrong_type", map[string]any{"id": "abc"}, "id", 0},
		{"nil_args", nil, "id", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			}
			if got := getIntArg(req, tt.key); got != tt.want {
				t.Errorf("getIntArg() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMarshalResult(t *testing.T) {
	result, err := marshalResult([]string{"a", "b"})
	if err != nil {
		t.Fatalf("marshalResult error: %v", err)
	}
	if result == nil {
		t.Fatal("marshalResult returned nil")
	}
	if result.IsError {
		t.Error("marshalResult should not set IsError")
	}
}

func TestErrorResult(t *testing.T) {
	result := errorResult(context.DeadlineExceeded)
	if result == nil {
		t.Fatal("errorResult returned nil")
	}
	if !result.IsError {
		t.Error("errorResult should set IsError=true")
	}
}
