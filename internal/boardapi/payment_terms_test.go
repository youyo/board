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

// newPaymentTermsMockClient returns a boardapi.Client whose HTTP client routes
// every request through rt.
func newPaymentTermsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListPaymentTermsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served.
// M56: ListPaymentTermsRaw(ctx, PaymentTermListOptions) に更新。
func TestListPaymentTermsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"name":"A","memo":"m","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newPaymentTermsMockClient(rt)

	raw, _, err := client.ListPaymentTermsRaw(context.Background(), boardapi.PaymentTermListOptions{})
	if err != nil {
		t.Fatalf("ListPaymentTermsRaw: %v", err)
	}
	if gotPath != "/v1/payment_terms" {
		t.Errorf("path = %s, want /v1/payment_terms", gotPath)
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

// U2: ListPaymentTermsRaw concatenates multiple pages into a single valid JSON array.
// M56: ListPaymentTermsRaw(ctx, PaymentTermListOptions{PerPage: 2}) に更新。
func TestListPaymentTermsRaw_MultiPage(t *testing.T) {
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
	client := newPaymentTermsMockClient(rt)

	raw, _, err := client.ListPaymentTermsRaw(context.Background(), boardapi.PaymentTermListOptions{PerPage: 2})
	if err != nil {
		t.Fatalf("ListPaymentTermsRaw: %v", err)
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

// U3: GetPaymentTermRaw returns body exactly as served (single object).
// M56: GetPaymentTermRaw(ctx, id) -> ([]byte, http.Header, error) に更新。
func TestGetPaymentTermRaw_Success(t *testing.T) {
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
	client := newPaymentTermsMockClient(rt)

	raw, _, err := client.GetPaymentTermRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetPaymentTermRaw: %v", err)
	}
	if gotPath != "/v1/payment_terms/42" {
		t.Errorf("path = %s, want /v1/payment_terms/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: ListPaymentTermsRaw with NameCont sends name_cont in the query.
// M56: SearchPaymentTermsRaw 廃止、ListPaymentTermsRaw(ctx, PaymentTermListOptions{NameCont: ...}) に更新。
func TestListPaymentTermsRaw_NameCont(t *testing.T) {
	page1 := `[{"id":7,"name":"keyword","memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedNameCont, observedGteq string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedNameCont = r.URL.Query().Get("name_cont")
		observedGteq = r.URL.Query().Get("updated_at_gteq")
		return jsonResp(page1), nil
	})
	client := newPaymentTermsMockClient(rt)

	raw, _, err := client.ListPaymentTermsRaw(context.Background(), boardapi.PaymentTermListOptions{
		NameCont:      "keyword",
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListPaymentTermsRaw: %v", err)
	}
	if observedNameCont != "keyword" {
		t.Errorf("name_cont = %q, want keyword", observedNameCont)
	}
	if observedGteq != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want %q", observedGteq, "2024-01-01 00:00:00")
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}

// U5: GetPaymentTermRaw on 404 returns *APIError{Code: APIErrorNotFound}.
// M56: GetPaymentTermRaw(ctx, id) -> ([]byte, http.Header, error) に更新。
func TestGetPaymentTermRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newPaymentTermsMockClient(rt)

	_, _, err := client.GetPaymentTermRaw(context.Background(), 99)
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
