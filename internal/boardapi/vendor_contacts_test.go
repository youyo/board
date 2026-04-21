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

// newVendorContactsMockClient returns a boardapi.Client whose HTTP client
// routes every request through rt. Mirrors newVendorBranchesMockClient (M14)
// to keep unit tests runnable in the sandbox where opening a local listener
// is denied. The shared roundTripperFunc and jsonResp helpers are defined in
// accounting_types_test.go (package-scope).
func newVendorContactsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListVendorContactsRaw returns the raw JSON array body byte-for-byte when
// a single page response is served. The exact JSON bytes the server emits must
// be preserved inside the returned array payload (element contents and order
// must not change) because StrictFieldDiff relies on the original response
// shape. URL path must be /v1/payee_contacts (the real BOARD API path, not
// /v1/vendor_contacts).
func TestListVendorContactsRaw_SinglePage(t *testing.T) {
	// 新スキーマ（M42 再設計）: vendor nested / last_name / first_name / title(*string) / department(*string) / email(*string) / note(*string) / archive_flg
	page1 := `[{"id":1,"vendor":{"id":10,"name":"Vendor X","name_disp":"Vendor X","custom_no":"VX01"},"last_name":"Yamada","first_name":"Taro","honorific_title":"様","title":"部長","department":"営業部","email":"taro@example.com","note":"note","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newVendorContactsMockClient(rt)

	raw, err := client.ListVendorContactsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListVendorContactsRaw: %v", err)
	}
	// Real BOARD API path is /v1/payee_contacts, not /v1/vendor_contacts.
	if gotPath != "/v1/payee_contacts" {
		t.Errorf("path = %s, want /v1/payee_contacts", gotPath)
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
	// 新スキーマのキーを検証（旧: vendor_id/vendor_branch_id/name/name_kana/phone/memo は廃止）
	wantKeys := []string{
		"id", "vendor", "last_name", "first_name", "honorific_title",
		"title", "department", "email", "note", "archive_flg",
		"updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListVendorContactsRaw concatenates multiple pages into a single valid
// JSON array. per_page=2 forces pagination; server returns 2 items on page 1
// and 1 item on page 2. Result must be a JSON array of 3 items.
func TestListVendorContactsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"last_name":"A","first_name":"","honorific_title":"","title":null,"department":null,"email":null,"note":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"last_name":"B","first_name":"","honorific_title":"","title":null,"department":null,"email":null,"note":null,"archive_flg":0,"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"vendor":{"id":11,"name":"VY","name_disp":"VY","custom_no":""},"last_name":"C","first_name":"","honorific_title":"","title":null,"department":null,"email":null,"note":null,"archive_flg":0,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newVendorContactsMockClient(rt)

	raw, err := client.ListVendorContactsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListVendorContactsRaw: %v", err)
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

// U3: GetVendorContactRaw returns body exactly as served (single object).
// Path must be /v1/payee_contacts/42 (real BOARD API path).
func TestGetVendorContactRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"vendor":{"id":10,"name":"Vendor X","name_disp":"Vendor X","custom_no":"VX01"},"last_name":"Yamada","first_name":"Taro","honorific_title":"様","title":"部長","department":"営業部","email":"taro@example.com","note":"note","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newVendorContactsMockClient(rt)

	raw, err := client.GetVendorContactRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetVendorContactRaw: %v", err)
	}
	// Real BOARD API path is /v1/payee_contacts/{id}, not /v1/vendor_contacts/{id}.
	if gotPath != "/v1/payee_contacts/42" {
		t.Errorf("path = %s, want /v1/payee_contacts/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetVendorContactRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetVendorContactRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newVendorContactsMockClient(rt)

	_, err := client.GetVendorContactRaw(context.Background(), 99)
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

// U5: SearchVendorContactsRaw sends all four parameters (VendorID, Name, Email,
// UpdatedAtFrom) in the query. VendorContactSearchParams has 4 fields — one
// more than M14's VendorBranchSearchParams (VendorID, Name, UpdatedAtFrom).
// This test ensures all four query params are correctly encoded.
func TestSearchVendorContactsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"vendor":{"id":123,"name":"VZ","name_disp":"VZ","custom_no":""},"last_name":"keyword","first_name":"","honorific_title":"","title":null,"department":null,"email":"test@example.com","note":null,"archive_flg":0,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedVendorID, observedName, observedEmail, observedUpdatedFrom string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedVendorID = r.URL.Query().Get("vendor_id")
		observedName = r.URL.Query().Get("name")
		observedEmail = r.URL.Query().Get("email")
		observedUpdatedFrom = r.URL.Query().Get("updated_at_from")
		return jsonResp(page1), nil
	})
	client := newVendorContactsMockClient(rt)

	raw, err := client.SearchVendorContactsRaw(context.Background(), boardapi.VendorContactSearchParams{
		VendorID:      123,
		Name:          "keyword",
		Email:         "test@example.com",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
	})
	if err != nil {
		t.Fatalf("SearchVendorContactsRaw: %v", err)
	}

	if observedVendorID != "123" {
		t.Errorf("vendor_id = %s, want 123", observedVendorID)
	}
	if observedName != "keyword" {
		t.Errorf("name = %s, want keyword", observedName)
	}
	if observedEmail != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", observedEmail)
	}
	if observedUpdatedFrom != "2024-01-01T00:00:00+09:00" {
		t.Errorf("updated_at_from = %s, want 2024-01-01T00:00:00+09:00", observedUpdatedFrom)
	}

	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("SearchVendorContactsRaw returned invalid JSON: %v\nraw=%s", err, string(raw))
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
