package boardapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newProjectCostsMockClient returns a boardapi.Client whose HTTP client routes
// every request through rt. Mirrors newContactsMockClient (M10) /
// newClientBranchesMockClient (M09) to keep unit tests runnable in the sandbox
// where opening a local listener is denied. The shared roundTripperFunc and
// jsonResp helpers are defined in accounting_types_test.go (package-scope).
func newProjectCostsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListProjectCostsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. All 8 ProjectCostEntity keys must survive
// the round trip so StrictFieldDiff can later detect any unmapped BOARD API
// keys in E2E. Note `amount` is a float64 in the Entity (unlike the int fields
// of users / contacts) — the mock response uses a decimal value to exercise
// that path.
func TestListProjectCostsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"project_id":100,"name":"Labor cost","cost_type":"labor","amount":12345.67,"memo":"engineer A","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newProjectCostsMockClient(rt)

	raw, err := client.ListProjectCostsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListProjectCostsRaw: %v", err)
	}
	if gotPath != "/v1/project_costs" {
		t.Errorf("path = %s, want /v1/project_costs", gotPath)
	}
	if gotQuery.Get("page") != "1" {
		t.Errorf("page = %s, want 1", gotQuery.Get("page"))
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v\nraw=%s", err, string(raw))
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
	got := arr[0]
	// All 8 ProjectCostEntity json tags must survive round-tripping so
	// StrictFieldDiff can detect any unmapped BOARD API keys.
	wantKeys := []string{
		"id", "project_id", "name", "cost_type",
		"amount", "memo", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListProjectCostsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListProjectCostsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"project_id":100,"name":"A","cost_type":"labor","amount":1000.0,"memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"project_id":100,"name":"B","cost_type":"material","amount":2000.5,"memo":"","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"project_id":101,"name":"C","cost_type":"outsourcing","amount":3000.75,"memo":"","updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
	}
	var pageCount int
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		pageCount++
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("per_page")
		if perPage != "2" {
			t.Errorf("expected per_page=2, got %s", perPage)
		}
		switch page {
		case "1":
			return jsonResp(fmt.Sprintf("[%s]", strings.Join(page1Items, ","))), nil
		case "2":
			return jsonResp(fmt.Sprintf("[%s]", strings.Join(page2Items, ","))), nil
		default:
			t.Errorf("unexpected page: %s", page)
			return jsonResp("[]"), nil
		}
	})
	client := newProjectCostsMockClient(rt)

	raw, err := client.ListProjectCostsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListProjectCostsRaw: %v", err)
	}
	if pageCount != 2 {
		t.Errorf("expected 2 page requests, got %d", pageCount)
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v\nraw=%s", err, string(raw))
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}
	for i, want := range []float64{1, 2, 3} {
		if got, _ := arr[i]["id"].(float64); got != want {
			t.Errorf("arr[%d].id = %v, want %v", i, got, want)
		}
	}
}

// U3: GetProjectCostRaw returns body exactly as served (single object).
func TestGetProjectCostRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"project_id":100,"name":"Material cost","cost_type":"material","amount":9999.99,"memo":"note","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newProjectCostsMockClient(rt)

	raw, err := client.GetProjectCostRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetProjectCostRaw: %v", err)
	}
	if gotPath != "/v1/project_costs/42" {
		t.Errorf("path = %s, want /v1/project_costs/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetProjectCostRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetProjectCostRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.GetProjectCostRaw(context.Background(), 99)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var apiErr *boardapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != boardapi.APIErrorNotFound {
		t.Errorf("APIError.Code = %v, want %v", apiErr.Code, boardapi.APIErrorNotFound)
	}
}

// U5: SearchProjectCostsRaw sends project_id in the query and returns the
// body. Unlike contacts (client_id/name/email, 3 filters) or client_branches
// (client_id/name, 2 filters), project_costs exposes only a single hierarchical
// filter `project_id`. This asserts encoding of that lone parameter.
func TestSearchProjectCostsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"project_id":123,"name":"x","cost_type":"labor","amount":100.0,"memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedProjectID, observedName string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedProjectID = r.URL.Query().Get("project_id")
		observedName = r.URL.Query().Get("name")
		return jsonResp(page1), nil
	})
	client := newProjectCostsMockClient(rt)

	raw, err := client.SearchProjectCostsRaw(context.Background(), boardapi.ProjectCostSearchParams{
		ProjectID: 123,
	})
	if err != nil {
		t.Fatalf("SearchProjectCostsRaw: %v", err)
	}
	if observedProjectID != "123" {
		t.Errorf("query project_id = %q, want 123", observedProjectID)
	}
	// project_costs does not support a `name` filter (unlike contacts /
	// client_branches). Verify the param is not emitted even if callers
	// happen to share the ContactSearchParams mental model.
	if observedName != "" {
		t.Errorf("query name should be absent for project_costs, got %q", observedName)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
