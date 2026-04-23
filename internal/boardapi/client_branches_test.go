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

// newClientBranchesMockClient returns a boardapi.Client whose HTTP client
// routes every request through rt. Mirrors newUsersMockClient (M08) /
// newGroupsMockClient (M07) / newPurchaseTypesMockClient (M06) to keep unit
// tests runnable in the sandbox where opening a local listener is denied. The
// shared roundTripperFunc and jsonResp helpers are defined in
// accounting_types_test.go (package-scope).
func newClientBranchesMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListClientBranchesRaw returns the raw JSON array body byte-for-byte when
// a single page response is served. The exact JSON bytes the server emits must
// be preserved inside the returned array payload (element contents and order
// must not change) because StrictFieldDiff relies on the original response
// shape.
// M39: mock JSON updated to real API schema (client nested, zip/pref/address1/address2/tel/archive_flg).
// M52: ListClientBranchesRaw シグネチャを ClientBranchListOptions に更新。
func TestListClientBranchesRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"client":{"id":10,"name":"株式会社テスト","name_disp":"テスト","custom_no":""},"name":"Head Office","zip":"100-0001","pref":"東京都","address1":"千代田区1-1","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newClientBranchesMockClient(rt)

	raw, _, err := client.ListClientBranchesRaw(context.Background(), boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("ListClientBranchesRaw: %v", err)
	}
	if gotPath != "/v1/client_branches" {
		t.Errorf("path = %s, want /v1/client_branches", gotPath)
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
		"id", "client", "name", "zip", "pref",
		"address1", "address2", "tel", "fax", "archive_flg",
		"updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListClientBranchesRaw concatenates multiple pages into a single valid
// JSON array. per_page=2 forces pagination; server returns 2 items on page 1
// and 1 item on page 2. Result must be a JSON array of 3 items.
// M39: mock JSON updated to real API schema.
// M52: ListClientBranchesRaw シグネチャを ClientBranchListOptions に更新。
func TestListClientBranchesRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"client":{"id":10,"name":"A社","name_disp":"A","custom_no":""},"name":"A","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"client":{"id":10,"name":"A社","name_disp":"A","custom_no":""},"name":"B","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"client":{"id":11,"name":"B社","name_disp":"B","custom_no":""},"name":"C","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newClientBranchesMockClient(rt)

	raw, _, err := client.ListClientBranchesRaw(context.Background(), boardapi.ClientBranchListOptions{PerPage: 2})
	if err != nil {
		t.Fatalf("ListClientBranchesRaw: %v", err)
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

// U3: GetClientBranchRaw returns body exactly as served (single object).
// M39: mock JSON updated to real API schema.
// M52: GetClientBranchRaw がヘッダーも返すシグネチャに更新。
func TestGetClientBranchRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"client":{"id":10,"name":"株式会社テスト","name_disp":"テスト","custom_no":""},"name":"Branch","zip":"100-0002","pref":"東京都","address1":"千代田区1-2","address2":"テストビル2F","tel":"03-1111-2222","fax":"03-1111-2223","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newClientBranchesMockClient(rt)

	raw, _, err := client.GetClientBranchRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetClientBranchRaw: %v", err)
	}
	if gotPath != "/v1/client_branches/42" {
		t.Errorf("path = %s, want /v1/client_branches/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetClientBranchRaw on 404 returns *APIError{Code: APIErrorNotFound}.
// M52: GetClientBranchRaw がヘッダーも返すシグネチャに更新。
func TestGetClientBranchRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newClientBranchesMockClient(rt)

	_, _, err := client.GetClientBranchRaw(context.Background(), 99)
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

// U5: ListClientBranchesRaw sends client_id_eq / name_cont in the query and returns body.
// M52: 旧 SearchClientBranchesRaw + ClientBranchSearchParams を
// ListClientBranchesRaw + ClientBranchListOptions に置き換え。
func TestListClientBranchesRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"client":{"id":123,"name":"株式会社テスト","name_disp":"テスト","custom_no":""},"name":"keyword","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedClientID, observedName string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedClientID = r.URL.Query().Get("client_id_eq")
		observedName = r.URL.Query().Get("name_cont")
		return jsonResp(page1), nil
	})
	client := newClientBranchesMockClient(rt)

	raw, _, err := client.ListClientBranchesRaw(context.Background(), boardapi.ClientBranchListOptions{
		ClientIDEq: 123,
		NameCont:   "keyword",
	})
	if err != nil {
		t.Fatalf("ListClientBranchesRaw: %v", err)
	}
	if observedClientID != "123" {
		t.Errorf("query client_id_eq = %q, want 123", observedClientID)
	}
	if observedName != "keyword" {
		t.Errorf("query name_cont = %q, want keyword", observedName)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
