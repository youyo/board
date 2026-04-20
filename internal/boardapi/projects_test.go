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

// newProjectsMockClient returns a boardapi.Client whose HTTP client routes every
// request through rt. Mirrors newClientsMockClient (M12) /
// newProjectCostsMockClient (M11) to keep unit tests runnable in the sandbox
// where opening a local listener is denied. The shared roundTripperFunc and
// jsonResp helpers are defined in accounting_types_test.go (package-scope).
func newProjectsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListProjectsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. All 10 base ProjectEntity json keys must
// survive the round trip so StrictFieldDiff can later detect any unmapped
// BOARD API keys in E2E. Optional DocumentSummary pointers (estimate/order/
// delivery/invoice/receipt) are omitted in the List variant — they appear
// only when response_group is specified on Get.
func TestListProjectsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"client_id":10,"name":"Project A","code":"P001","status":"active","start_date":"2024-01-01","end_date":"2024-12-31","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.ListProjectsRaw(context.Background())
	if err != nil {
		t.Fatalf("ListProjectsRaw: %v", err)
	}
	if gotPath != "/v1/projects" {
		t.Errorf("path = %s, want /v1/projects", gotPath)
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
	// All 10 ProjectEntity base json tags must survive round-tripping so
	// StrictFieldDiff can detect any unmapped BOARD API keys.
	wantKeys := []string{
		"id", "client_id", "name", "code", "status",
		"start_date", "end_date", "memo", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListProjectsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListProjectsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"client_id":10,"name":"A","code":"P001","status":"active","start_date":"2024-01-01","end_date":"2024-12-31","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"client_id":10,"name":"B","code":"P002","status":"active","start_date":"2024-01-02","end_date":"2024-12-31","memo":"","updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"client_id":11,"name":"C","code":"P003","status":"closed","start_date":"2024-01-03","end_date":"2024-12-31","memo":"","updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newProjectsMockClient(rt)

	raw, err := client.ListProjectsRaw(context.Background(), boardapi.WithPerPage(2))
	if err != nil {
		t.Fatalf("ListProjectsRaw: %v", err)
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

// U3: GetProjectRaw returns body exactly as served (single object).
func TestGetProjectRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"client_id":10,"name":"Project X","code":"PX","status":"active","start_date":"2024-01-01","end_date":"2024-12-31","memo":"","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.GetProjectRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetProjectRaw: %v", err)
	}
	if gotPath != "/v1/projects/42" {
		t.Errorf("path = %s, want /v1/projects/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetProjectRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetProjectRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newProjectsMockClient(rt)

	_, err := client.GetProjectRaw(context.Background(), 99)
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

// U5: SearchProjectsRaw sends all five parameters (ClientID, Name, Status,
// UpdatedAtFrom, ResponseGroup) in the query. ProjectSearchParams has the
// richest surface across all M02-M12 resources — M08 users had 3, M12 clients
// had 2, M11 project_costs had 1 — so parameter encoding is worth its own test.
func TestSearchProjectsRaw_QueryParams(t *testing.T) {
	page1 := `[{"id":7,"client_id":100,"name":"matched","code":"","status":"active","start_date":"2024-01-01","end_date":"2024-12-31","memo":"","updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedClientID, observedName, observedStatus, observedUpdatedFrom, observedResponseGroup string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedClientID = r.URL.Query().Get("client_id")
		observedName = r.URL.Query().Get("name")
		observedStatus = r.URL.Query().Get("status")
		observedUpdatedFrom = r.URL.Query().Get("updated_at_from")
		observedResponseGroup = r.URL.Query().Get("response_group")
		return jsonResp(page1), nil
	})
	client := newProjectsMockClient(rt)

	raw, err := client.SearchProjectsRaw(context.Background(), boardapi.ProjectSearchParams{
		ClientID:      100,
		Name:          "matched",
		Status:        "active",
		UpdatedAtFrom: "2024-01-01T00:00:00+09:00",
		ResponseGroup: "estimate",
	})
	if err != nil {
		t.Fatalf("SearchProjectsRaw: %v", err)
	}
	if observedClientID != "100" {
		t.Errorf("query client_id = %q, want 100", observedClientID)
	}
	if observedName != "matched" {
		t.Errorf("query name = %q, want matched", observedName)
	}
	if observedStatus != "active" {
		t.Errorf("query status = %q, want active", observedStatus)
	}
	if observedUpdatedFrom != "2024-01-01T00:00:00+09:00" {
		t.Errorf("query updated_at_from = %q, want 2024-01-01T00:00:00+09:00", observedUpdatedFrom)
	}
	if observedResponseGroup != "estimate" {
		t.Errorf("query response_group = %q, want estimate", observedResponseGroup)
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}

// U6: GetProjectWithGroupRaw sends the response_group value as a query
// parameter for every supported group (estimate/order/delivery/invoice/
// receipt/all). When the response_group arg is empty, the request must NOT
// include a response_group query at all — behaving identically to GetProjectRaw.
// This test is M13-specific: no prior milestone exercises a per-group query
// surface, and BOARD API compliance hinges on each group being individually
// addressable.
func TestGetProjectWithGroupRaw_QueryParam(t *testing.T) {
	body := []byte(`{"id":42,"client_id":10,"name":"P","code":"","status":"active","start_date":"","end_date":"","memo":"","updated_at":"","created_at":""}`)
	groups := []string{"estimate", "order", "delivery", "invoice", "receipt", "all"}
	for _, group := range groups {
		t.Run(group, func(t *testing.T) {
			var gotPath, gotRawQuery string
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				gotRawQuery = r.URL.RawQuery
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			})
			client := newProjectsMockClient(rt)
			_, err := client.GetProjectWithGroupRaw(context.Background(), 42, group)
			if err != nil {
				t.Fatalf("GetProjectWithGroupRaw(%s): %v", group, err)
			}
			if gotPath != "/v1/projects/42" {
				t.Errorf("path = %s, want /v1/projects/42", gotPath)
			}
			wantQuery := "response_group=" + group
			if gotRawQuery != wantQuery {
				t.Errorf("raw query = %q, want %q", gotRawQuery, wantQuery)
			}
		})
	}

	// Empty response_group: no query parameter must be appended.
	t.Run("empty", func(t *testing.T) {
		var gotPath, gotRawQuery string
		rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotRawQuery = r.URL.RawQuery
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		})
		client := newProjectsMockClient(rt)
		_, err := client.GetProjectWithGroupRaw(context.Background(), 42, "")
		if err != nil {
			t.Fatalf("GetProjectWithGroupRaw(empty): %v", err)
		}
		if gotPath != "/v1/projects/42" {
			t.Errorf("path = %s, want /v1/projects/42", gotPath)
		}
		if gotRawQuery != "" {
			t.Errorf("raw query = %q, want empty (no response_group when arg is empty)", gotRawQuery)
		}
	})
}
