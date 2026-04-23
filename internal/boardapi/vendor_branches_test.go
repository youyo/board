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
// a single page response is served. URL path must be /v1/payee_branches.
// M55: ListVendorBranchesRaw(ctx, VendorBranchListOptions) に更新。
func TestListVendorBranchesRaw_SinglePage(t *testing.T) {
	// 新スキーマ（M41 再設計）: vendor nested / zip / pref / address1 / address2 / tel / fax(*string) / archive_flg
	page1 := `[{"id":1,"vendor":{"id":10,"name":"Vendor X","name_disp":"Vendor X","custom_no":"VX01"},"name":"Main Branch","zip":"100-0001","pref":"東京都","address1":"千代田区","address2":"","tel":"03-0000-0000","fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newVendorBranchesMockClient(rt)

	raw, _, err := client.ListVendorBranchesRaw(context.Background(), boardapi.VendorBranchListOptions{})
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
	// 新スキーマのキーを検証（旧: vendor_id/postal_code/address/phone/memo は廃止）
	wantKeys := []string{
		"id", "vendor", "name", "zip", "pref",
		"address1", "address2", "tel", "fax", "archive_flg", "updated_at", "created_at",
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
// M55: ListVendorBranchesRaw(ctx, VendorBranchListOptions{PerPage: 2}) に更新。
func TestListVendorBranchesRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"name":"A","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"name":"B","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"vendor":{"id":11,"name":"VY","name_disp":"VY","custom_no":""},"name":"C","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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

	raw, _, err := client.ListVendorBranchesRaw(context.Background(), boardapi.VendorBranchListOptions{PerPage: 2})
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
// M55: GetVendorBranchRaw(ctx, id) -> ([]byte, http.Header, error) に更新。
func TestGetVendorBranchRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"vendor":{"id":10,"name":"Vendor X","name_disp":"Vendor X","custom_no":"VX01"},"name":"Branch","zip":"100-0002","pref":"東京都","address1":"港区","address2":"","tel":"03-1111-2222","fax":"03-1111-2223","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
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

	raw, _, err := client.GetVendorBranchRaw(context.Background(), 42)
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
// M55: GetVendorBranchRaw(ctx, id) -> ([]byte, http.Header, error) に更新。
func TestGetVendorBranchRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newVendorBranchesMockClient(rt)

	_, _, err := client.GetVendorBranchRaw(context.Background(), 99)
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

// U5: ListVendorBranchesRaw with PayeeIDEq sends payee_id_eq in the query.
// M55: VendorBranchListOptions{PayeeIDEq: 123} に更新（Ransack 準拠）。
// 注意: BOARD API の実際のパラメータ名は E2E テストで確認すること。
func TestListVendorBranchesRaw_PayeeIDEq(t *testing.T) {
	page1 := `[{"id":7,"vendor":{"id":123,"name":"VZ","name_disp":"VZ","custom_no":""},"name":"keyword","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedPayeeIDEq, observedNameCont string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedPayeeIDEq = r.URL.Query().Get("payee_id_eq")
		observedNameCont = r.URL.Query().Get("name_cont")
		return jsonResp(page1), nil
	})
	client := newVendorBranchesMockClient(rt)

	raw, _, err := client.ListVendorBranchesRaw(context.Background(), boardapi.VendorBranchListOptions{
		PayeeIDEq: 123,
		NameCont:  "keyword",
	})
	if err != nil {
		t.Fatalf("ListVendorBranchesRaw: %v", err)
	}
	if observedPayeeIDEq != "123" {
		t.Errorf("query payee_id_eq = %q, want 123", observedPayeeIDEq)
	}
	if observedNameCont != "keyword" {
		t.Errorf("query name_cont = %q, want keyword", observedNameCont)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
