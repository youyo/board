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

// newVendorBranchesMockClient returns a boardapi.Client whose HTTP client
// routes every request through rt. Mirrors newClientBranchesMockClient (M09) /
// newProjectsMockClient (M13) to keep unit tests runnable in the sandbox where
// opening a local listener is denied. The shared roundTripperFunc and jsonResp
// helpers are defined in accounting_types_test.go (package-scope).
func newVendorBranchesMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListVendorBranchesRaw returns the raw JSON array body byte-for-byte when
// a single page response is served. The exact JSON bytes the server emits must
// be preserved inside the returned array payload (element contents and order
// must not change) because StrictFieldDiff relies on the original response
// shape. URL path must be /v1/payee_branches (the real BOARD API path, not
// /v1/vendor_branches).
func TestListVendorBranchesRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"vendor_id":10,"name":"Main Branch","postal_code":"100-0001","address":"Tokyo","phone":"03-0000-0000","fax":"03-0000-0001","memo":"main","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newVendorBranchesMockClient(rt)

	raw, err := client.ListVendorBranchesRaw(context.Background())
	if err != nil {
		t.Fatalf("ListVendorBranchesRaw: %v", err)
	}
	// Real BOARD API path is /v1/payee_branches, not /v1/vendor_branches.
	if gotPath != "/v1/payee_branches" {
		t.Errorf("path = %s, want /v1/payee_branches", gotPath)
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
	wantKeys := []string{
		"id", "vendor_id", "name", "postal_code", "address",
		"phone", "fax", "memo", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListVendorBranchesRaw concatenates multiple pages into a single valid
// JSON array. per_page=2 forces pagination; server returns 2 items on page 1
// and 1 item on page 2. Result must be a JSON array of 3 items.
func TestListVendorBranchesRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"vendor_id":10,"name":"A","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"vendor_id":10,"name":"B","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"vendor_id":11,"name":"C","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newVendorBranchesMockClient(rt)

	raw, err := client.ListVendorBranchesRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListVendorBranchesRaw: %v", err)
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

// U3: GetVendorBranchRaw returns body exactly as served (single object).
// Path must be /v1/payee_branches/42 (real BOARD API path).
func TestGetVendorBranchRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"vendor_id":10,"name":"Branch","postal_code":"100-0002","address":"Tokyo 2","phone":"03-1111-2222","fax":"03-1111-2223","memo":"sub","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newVendorBranchesMockClient(rt)

	raw, err := client.GetVendorBranchRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetVendorBranchRaw: %v", err)
	}
	// Real BOARD API path is /v1/payee_branches/{id}, not /v1/vendor_branches/{id}.
	if gotPath != "/v1/payee_branches/42" {
		t.Errorf("path = %s, want /v1/payee_branches/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetVendorBranchRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetVendorBranchRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newVendorBranchesMockClient(rt)

	_, err := client.GetVendorBranchRaw(context.Background(), 99)
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

// U5: SearchVendorBranchesRaw sends all three parameters (VendorID, Name,
// UpdatedAtFrom) in the query. VendorBranchSearchParams has 3 fields — one
// more than M09's ClientBranchSearchParams (ClientID, Name). This test ensures
// all three query params are correctly encoded.
func TestSearchVendorBranchesRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"vendor_id":123,"name":"keyword","postal_code":"","address":"","phone":"","fax":"","memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedVendorID, observedName, observedUpdatedFrom string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedVendorID = r.URL.Query().Get("vendor_id")
		observedName = r.URL.Query().Get("name")
		observedUpdatedFrom = r.URL.Query().Get("updated_at_from")
		return jsonResp(page1), nil
	})
	client := newVendorBranchesMockClient(rt)

	raw, err := client.SearchVendorBranchesRaw(context.Background(), boardapi.VendorBranchSearchParams{
		VendorID:      123,
		Name:          "keyword",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
	})
	if err != nil {
		t.Fatalf("SearchVendorBranchesRaw: %v", err)
	}
	if observedVendorID != "123" {
		t.Errorf("query vendor_id = %q, want 123", observedVendorID)
	}
	if observedName != "keyword" {
		t.Errorf("query name = %q, want keyword", observedName)
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
