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

// U1: TestProjectCostEntity_UnmarshalGet_AllFields — 実 dump（8 フィールド）を
// デシリアライズして全フィールドを検証。
// 実データ根拠: tmp/e2e-artifacts/project_costs_33291004.json
func TestProjectCostEntity_UnmarshalGet_AllFields(t *testing.T) {
	raw := []byte(`{"id":33291004,"project_id":85079735,"description":"S2への支払い","cost":400000,"invoice_date":"2020-02-01","payment_date":"2020-02-29","created_at":"2020-06-15T11:02:58.000+09:00","updated_at":"2020-06-15T11:02:58.000+09:00"}`)

	var pc boardapi.ProjectCostEntity
	if err := json.Unmarshal(raw, &pc); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if pc.ID != 33291004 {
		t.Errorf("ID = %d, want 33291004", pc.ID)
	}
	if pc.ProjectID != 85079735 {
		t.Errorf("ProjectID = %d, want 85079735", pc.ProjectID)
	}
	if pc.Description != "S2への支払い" {
		t.Errorf("Description = %q, want %q", pc.Description, "S2への支払い")
	}
	if pc.Cost != 400000 {
		t.Errorf("Cost = %d, want 400000", pc.Cost)
	}
	if pc.InvoiceDate == nil || *pc.InvoiceDate != "2020-02-01" {
		t.Errorf("InvoiceDate = %v, want %q", pc.InvoiceDate, "2020-02-01")
	}
	if pc.PaymentDate == nil || *pc.PaymentDate != "2020-02-29" {
		t.Errorf("PaymentDate = %v, want %q", pc.PaymentDate, "2020-02-29")
	}
	if pc.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if pc.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
}

// U2: TestProjectCostEntity_UnmarshalWithNullDates — invoice_date / payment_date
// が null の場合、対応するポインタフィールドが nil になることを確認。
func TestProjectCostEntity_UnmarshalWithNullDates(t *testing.T) {
	raw := []byte(`{"id":1,"project_id":10,"description":"テスト","cost":0,"invoice_date":null,"payment_date":null,"created_at":"2024-01-01T00:00:00+09:00","updated_at":"2024-01-01T00:00:00+09:00"}`)

	var pc boardapi.ProjectCostEntity
	if err := json.Unmarshal(raw, &pc); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if pc.InvoiceDate != nil {
		t.Errorf("InvoiceDate should be nil for null JSON, got %v", *pc.InvoiceDate)
	}
	if pc.PaymentDate != nil {
		t.Errorf("PaymentDate should be nil for null JSON, got %v", *pc.PaymentDate)
	}
}

// U3: TestProjectCostSearchParams_QueryEncoding — ProjectID=85079735 が
// クエリパラメータ project_id=85079735 としてエンコードされることを確認。
func TestProjectCostSearchParams_QueryEncoding(t *testing.T) {
	page1 := `[{"id":1,"project_id":85079735,"description":"x","cost":0,"invoice_date":null,"payment_date":null,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2024-01-01T00:00:00+09:00"}]`
	var observedProjectID string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedProjectID = r.URL.Query().Get("project_id")
		return jsonResp(page1), nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.SearchProjectCostsRaw(context.Background(), boardapi.ProjectCostSearchParams{
		ProjectID: 85079735,
	})
	if err != nil {
		t.Fatalf("SearchProjectCostsRaw: %v", err)
	}
	if observedProjectID != "85079735" {
		t.Errorf("query project_id = %q, want 85079735", observedProjectID)
	}
}

// ListProjectCostsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. All 8 ProjectCostEntity keys must survive
// the round trip so StrictFieldDiff can later detect any unmapped BOARD API
// keys in E2E.
func TestListProjectCostsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"project_id":100,"description":"開発費","cost":12345,"invoice_date":"2024-01-01","payment_date":null,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
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
		"id", "project_id", "description", "cost",
		"invoice_date", "payment_date", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// ListProjectCostsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListProjectCostsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"project_id":100,"description":"A","cost":1000,"invoice_date":"2024-01-01","payment_date":null,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"project_id":100,"description":"B","cost":2000,"invoice_date":null,"payment_date":"2024-01-02","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"project_id":101,"description":"C","cost":3000,"invoice_date":null,"payment_date":null,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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

// GetProjectCostRaw returns body exactly as served (single object).
func TestGetProjectCostRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"project_id":100,"description":"外注費","cost":9999,"invoice_date":"2024-01-01","payment_date":"2024-02-01","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
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

// GetProjectCostRaw on 404 returns *APIError{Code: APIErrorNotFound}.
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

// SearchProjectCostsRaw sends project_id in the query and returns the
// body. Unlike contacts (client_id/name/email, 3 filters) or client_branches
// (client_id/name, 2 filters), project_costs exposes only a single hierarchical
// filter `project_id`. This asserts encoding of that lone parameter.
func TestSearchProjectCostsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"project_id":123,"description":"x","cost":100,"invoice_date":null,"payment_date":null,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
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
