package boardapi_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// parseQuery は URL クエリ文字列をパースして url.Values を返すテストヘルパ。
func parseQuery(t *testing.T, rawQuery string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q): %v", rawQuery, err)
	}
	return q
}

// newVendorsM55MockClient は M55 のテスト専用モッククライアントを返す。
func newVendorsM55MockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// M55-V1: buildVendorsQuery はゼロフィルタのとき page/per_page のみを送信する。
func TestBuildVendorsQuery_Zero(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorsM55MockClient(rt)

	_, err := client.ListVendors(context.Background(), boardapi.VendorListOptions{})
	if err != nil {
		t.Fatalf("ListVendors: %v", err)
	}
	// ゼロフィルタ: page=1 と per_page=50 のみ（フィルタパラメータなし）
	if gotQuery == "" {
		t.Fatalf("expected query string, got empty")
	}
	q := parseQuery(t, gotQuery)
	if q.Get("page") != "1" {
		t.Errorf("page = %q, want 1", q.Get("page"))
	}
	if q.Get("name_cont") != "" {
		t.Errorf("name_cont should be empty, got %q", q.Get("name_cont"))
	}
}

// M55-V2: buildVendorsQuery は NameCont を name_cont パラメータとして送信する。
func TestBuildVendorsQuery_NameCont(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorsM55MockClient(rt)

	_, err := client.ListVendors(context.Background(), boardapi.VendorListOptions{
		NameCont: "テスト",
	})
	if err != nil {
		t.Fatalf("ListVendors: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("name_cont") != "テスト" {
		t.Errorf("name_cont = %q, want テスト", q.Get("name_cont"))
	}
}

// M55-V3: buildVendorsQuery は UpdatedAtGteq / UpdatedAtLteq を送信する。
func TestBuildVendorsQuery_UpdatedAtRange(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorsM55MockClient(rt)

	_, err := client.ListVendors(context.Background(), boardapi.VendorListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
		UpdatedAtLteq: "2024-12-31 23:59:59",
	})
	if err != nil {
		t.Fatalf("ListVendors: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("updated_at_gteq") != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", q.Get("updated_at_gteq"))
	}
	if q.Get("updated_at_lteq") != "2024-12-31 23:59:59" {
		t.Errorf("updated_at_lteq = %q, want 2024-12-31 23:59:59", q.Get("updated_at_lteq"))
	}
}

// M55-V4: buildVendorsQuery は IncludeArchiveFlg=true を include_archive_flg=1 として送信する。
func TestBuildVendorsQuery_IncludeArchiveFlg(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorsM55MockClient(rt)

	trueVal := true
	_, err := client.ListVendors(context.Background(), boardapi.VendorListOptions{
		IncludeArchiveFlg: &trueVal,
	})
	if err != nil {
		t.Fatalf("ListVendors: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("include_archive_flg") != "1" {
		t.Errorf("include_archive_flg = %q, want 1", q.Get("include_archive_flg"))
	}
}

// M55-V5: ListVendors は *ListResult[VendorEntity] を返す。
func TestListVendors_ReturnsListResult(t *testing.T) {
	page1 := `[{"id":1,"name":"テスト商事","code":"V001","memo":"メモ","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonRespWithHeaders(page1, http.Header{"X-Total-Count": []string{"1"}}), nil
	})
	client := newVendorsM55MockClient(rt)

	result, err := client.ListVendors(context.Background(), boardapi.VendorListOptions{})
	if err != nil {
		t.Fatalf("ListVendors: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
	if result.Items[0].ID != 1 {
		t.Errorf("Items[0].ID = %d, want 1", result.Items[0].ID)
	}
}

// M55-V6: GetVendor は *ItemResult[VendorEntity] を返す。
func TestGetVendor_ReturnsItemResult(t *testing.T) {
	body := `{"id":42,"name":"テスト商事","code":"V042","memo":"メモ","updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(body), nil
	})
	client := newVendorsM55MockClient(rt)

	result, err := client.GetVendor(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetVendor: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ItemResult")
	}
	if result.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	if result.Item.ID != 42 {
		t.Errorf("Item.ID = %d, want 42", result.Item.ID)
	}
}
