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

// newContactsMockClient returns a boardapi.Client whose HTTP client routes
// every request through rt. Mirrors newUsersMockClient (M08) /
// newClientBranchesMockClient (M09) to keep unit tests runnable in the sandbox
// where opening a local listener is denied. The shared roundTripperFunc and
// jsonResp helpers are defined in accounting_types_test.go (package-scope).
func newContactsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListContactsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. The exact JSON bytes the server emits must
// be preserved inside the returned array payload (element contents and order
// must not change) because StrictFieldDiff relies on the original response
// shape. All 17 ContactEntity keys are expected to survive the round trip.
func TestListContactsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"client_id":10,"client_branch_id":0,"name":"Yamada Taro","name_kana":"ヤマダ タロウ","last_name":"Yamada","first_name":"Taro","honorific_title":"様","title":"部長","department":"Sales","email":"taro@example.com","phone":"03-0000-0000","note":"main contact","memo":"vip","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newContactsMockClient(rt)

	raw, err := client.ListContactsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListContactsRaw: %v", err)
	}
	if gotPath != "/v1/contacts" {
		t.Errorf("path = %s, want /v1/contacts", gotPath)
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
	// All 17 ContactEntity json tags must survive round-tripping so
	// StrictFieldDiff can detect any unmapped BOARD API keys.
	wantKeys := []string{
		"id", "client_id", "client_branch_id", "name", "name_kana",
		"last_name", "first_name", "honorific_title", "title", "department",
		"email", "phone", "note", "memo", "archive_flg",
		"updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListContactsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListContactsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"client_id":10,"client_branch_id":0,"name":"A","name_kana":"","last_name":"","first_name":"","honorific_title":"","title":"","department":"","email":"","phone":"","note":"","memo":"","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"client_id":10,"client_branch_id":0,"name":"B","name_kana":"","last_name":"","first_name":"","honorific_title":"","title":"","department":"","email":"","phone":"","note":"","memo":"","archive_flg":0,"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"client_id":11,"client_branch_id":0,"name":"C","name_kana":"","last_name":"","first_name":"","honorific_title":"","title":"","department":"","email":"","phone":"","note":"","memo":"","archive_flg":0,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newContactsMockClient(rt)

	raw, err := client.ListContactsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListContactsRaw: %v", err)
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

// U3: GetContactRaw returns body exactly as served (single object).
func TestGetContactRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"client_id":10,"client_branch_id":5,"name":"","name_kana":"","last_name":"Sato","first_name":"Hanako","honorific_title":"様","title":"","department":"","email":"hanako@example.com","phone":"","note":"","memo":"","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newContactsMockClient(rt)

	raw, err := client.GetContactRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetContactRaw: %v", err)
	}
	if gotPath != "/v1/contacts/42" {
		t.Errorf("path = %s, want /v1/contacts/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetContactRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetContactRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newContactsMockClient(rt)

	_, err := client.GetContactRaw(context.Background(), 99)
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

// U5: SearchContactsRaw sends client_id / name / email in the query and returns
// body. Unlike users (name/email/updated_at_from) or client_branches
// (client_id/name), contacts exposes 3 filters: client_id as a hierarchical
// filter plus name and email as keyword filters.
func TestSearchContactsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"client_id":123,"client_branch_id":0,"name":"keyword","name_kana":"","last_name":"","first_name":"","honorific_title":"","title":"","department":"","email":"x@y.z","phone":"","note":"","memo":"","archive_flg":0,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedClientID, observedName, observedEmail string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedClientID = r.URL.Query().Get("client_id")
		observedName = r.URL.Query().Get("name")
		observedEmail = r.URL.Query().Get("email")
		return jsonResp(page1), nil
	})
	client := newContactsMockClient(rt)

	raw, err := client.SearchContactsRaw(context.Background(), boardapi.ContactSearchParams{
		ClientID: 123,
		Name:     "keyword",
		Email:    "x@y.z",
	})
	if err != nil {
		t.Fatalf("SearchContactsRaw: %v", err)
	}
	if observedClientID != "123" {
		t.Errorf("query client_id = %q, want 123", observedClientID)
	}
	if observedName != "keyword" {
		t.Errorf("query name = %q, want keyword", observedName)
	}
	if observedEmail != "x@y.z" {
		t.Errorf("query email = %q, want x@y.z", observedEmail)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}
