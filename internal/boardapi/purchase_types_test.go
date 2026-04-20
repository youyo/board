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

// newPurchaseTypesMockClient returns a boardapi.Client whose HTTP client routes
// every request through rt. Mirrors newPaymentTermsMockClient (M04) and
// newProjectTypesMockClient (M03) to keep unit tests runnable in the sandbox
// where opening a local listener is denied. The shared roundTripperFunc and
// jsonResp helpers are defined in accounting_types_test.go (package-scope).
func newPurchaseTypesMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListPurchaseTypesRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. The exact JSON bytes the server emits must be
// preserved inside the returned array payload (element contents and order must
// not change) because StrictFieldDiff relies on the original response shape.
func TestListPurchaseTypesRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"name":"A","memo":"m","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newPurchaseTypesMockClient(rt)

	raw, err := client.ListPurchaseTypesRaw(context.Background())
	if err != nil {
		t.Fatalf("ListPurchaseTypesRaw: %v", err)
	}
	if gotPath != "/v1/expenditure_types" {
		t.Errorf("path = %s, want /v1/expenditure_types", gotPath)
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
	wantKeys := []string{"id", "name", "memo", "updated_at", "created_at"}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListPurchaseTypesRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and 1
// on page 2. Result must be a JSON array of 3 items, preserving original
// element JSON byte-for-byte.
func TestListPurchaseTypesRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"name":"A","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"name":"B","memo":"","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"name":"C","memo":"","updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newPurchaseTypesMockClient(rt)

	raw, err := client.ListPurchaseTypesRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListPurchaseTypesRaw: %v", err)
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

// U3: GetPurchaseTypeRaw returns body exactly as served (single object).
func TestGetPurchaseTypeRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"name":"Foo","memo":"mm","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newPurchaseTypesMockClient(rt)

	raw, err := client.GetPurchaseTypeRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetPurchaseTypeRaw: %v", err)
	}
	if gotPath != "/v1/expenditure_types/42" {
		t.Errorf("path = %s, want /v1/expenditure_types/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: SearchPurchaseTypesRaw sends name / updated_at_from in the query and returns body.
func TestSearchPurchaseTypesRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"name":"keyword","memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedName, observedFrom string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedName = r.URL.Query().Get("name")
		observedFrom = r.URL.Query().Get("updated_at_from")
		return jsonResp(page1), nil
	})
	client := newPurchaseTypesMockClient(rt)

	raw, err := client.SearchPurchaseTypesRaw(context.Background(), boardapi.PurchaseTypeSearchParams{
		Name:          "keyword",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
	})
	if err != nil {
		t.Fatalf("SearchPurchaseTypesRaw: %v", err)
	}
	if observedName != "keyword" {
		t.Errorf("query name = %q, want keyword", observedName)
	}
	if observedFrom != "2024-01-01T00:00:00+09:00" {
		t.Errorf("query updated_at_from = %q, want %q", observedFrom, "2024-01-01T00:00:00+09:00")
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}

// U5: GetPurchaseTypeRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetPurchaseTypeRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newPurchaseTypesMockClient(rt)

	_, err := client.GetPurchaseTypeRaw(context.Background(), 99)
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
