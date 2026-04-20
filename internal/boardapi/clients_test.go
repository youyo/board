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

// newClientsMockClient returns a boardapi.Client whose HTTP client routes every
// request through rt. Mirrors newProjectCostsMockClient (M11) /
// newContactsMockClient (M10) / newClientBranchesMockClient (M09) to keep unit
// tests runnable in the sandbox where opening a local listener is denied. The
// shared roundTripperFunc and jsonResp helpers are defined in
// accounting_types_test.go (package-scope).
func newClientsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListClientsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. All 6 ClientEntity keys must survive the
// round trip so StrictFieldDiff can later detect any unmapped BOARD API keys
// in E2E.
func TestListClientsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"name":"Client A","code":"C001","memo":"VIP","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newClientsMockClient(rt)

	raw, err := client.ListClientsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
	}
	if gotPath != "/v1/clients" {
		t.Errorf("path = %s, want /v1/clients", gotPath)
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
	// All 6 ClientEntity json tags must survive round-tripping so
	// StrictFieldDiff can detect any unmapped BOARD API keys.
	wantKeys := []string{
		"id", "name", "code", "memo", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListClientsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListClientsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"name":"A","code":"C001","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"name":"B","code":"C002","memo":"","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"name":"C","code":"C003","memo":"","updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newClientsMockClient(rt)

	raw, err := client.ListClientsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
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

// U3: GetClientRaw returns body exactly as served (single object).
func TestGetClientRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"name":"Client X","code":"CX","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newClientsMockClient(rt)

	raw, err := client.GetClientRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetClientRaw: %v", err)
	}
	if gotPath != "/v1/clients/42" {
		t.Errorf("path = %s, want /v1/clients/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetClientRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetClientRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newClientsMockClient(rt)

	_, err := client.GetClientRaw(context.Background(), 99)
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

// U5: SearchClientsRaw sends both Name and UpdatedAtFrom parameters in the
// query and returns the body. M08 users uses the same two-parameter surface.
func TestSearchClientsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"name":"matched","code":"","memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedName, observedUpdatedFrom string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedName = r.URL.Query().Get("name")
		observedUpdatedFrom = r.URL.Query().Get("updated_at_from")
		return jsonResp(page1), nil
	})
	client := newClientsMockClient(rt)

	raw, err := client.SearchClientsRaw(context.Background(), boardapi.ClientSearchParams{
		Name:          "matched",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
	})
	if err != nil {
		t.Fatalf("SearchClientsRaw: %v", err)
	}
	if observedName != "matched" {
		t.Errorf("query name = %q, want matched", observedName)
	}
	if observedUpdatedFrom != "2024-01-01T00:00:00+09:00" {
		t.Errorf("query updated_at_from = %q, want 2024-01-01T00:00:00+09:00", observedUpdatedFrom)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
