package boardapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newVendorBranchesM55MockClient は M55 のテスト専用モッククライアントを返す。
func newVendorBranchesM55MockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// M55-VB1: buildVendorBranchesQuery はゼロフィルタのとき page/per_page のみを送信する。
func TestBuildVendorBranchesQuery_Zero(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	_, err := client.ListVendorBranches(context.Background(), boardapi.VendorBranchListOptions{})
	if err != nil {
		t.Fatalf("ListVendorBranches: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("page") != "1" {
		t.Errorf("page = %q, want 1", q.Get("page"))
	}
	if q.Get("payee_id_eq") != "" {
		t.Errorf("payee_id_eq should be empty for zero filter, got %q", q.Get("payee_id_eq"))
	}
}

// M55-VB2: buildVendorBranchesQuery は PayeeIDEq を payee_id_eq パラメータとして送信する。
// 注意: BOARD API のリソース URL が /v1/payees であるため、Ransack FK は payee_id_eq と推定。
// E2E テストで実際のパラメータ名を確認すること。
func TestBuildVendorBranchesQuery_PayeeIDEq(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	_, err := client.ListVendorBranches(context.Background(), boardapi.VendorBranchListOptions{
		PayeeIDEq: 123,
	})
	if err != nil {
		t.Fatalf("ListVendorBranches: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("payee_id_eq") != "123" {
		t.Errorf("payee_id_eq = %q, want 123", q.Get("payee_id_eq"))
	}
}

// M55-VB3: buildVendorBranchesQuery は NameCont を name_cont パラメータとして送信する。
func TestBuildVendorBranchesQuery_NameCont(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	_, err := client.ListVendorBranches(context.Background(), boardapi.VendorBranchListOptions{
		NameCont: "東京",
	})
	if err != nil {
		t.Fatalf("ListVendorBranches: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("name_cont") != "東京" {
		t.Errorf("name_cont = %q, want 東京", q.Get("name_cont"))
	}
}

// M55-VB4: buildVendorBranchesQuery は UpdatedAtGteq を updated_at_gteq として送信する。
func TestBuildVendorBranchesQuery_UpdatedAtGteq(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	_, err := client.ListVendorBranches(context.Background(), boardapi.VendorBranchListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListVendorBranches: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("updated_at_gteq") != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", q.Get("updated_at_gteq"))
	}
}

// M55-VB5: ListVendorBranches は *ListResult[VendorBranchEntity] を返す。
func TestListVendorBranches_ReturnsListResult(t *testing.T) {
	page1 := `[{"id":1,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"name":"Branch A","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonRespWithHeaders(page1, http.Header{"X-Total-Count": []string{"1"}}), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	result, err := client.ListVendorBranches(context.Background(), boardapi.VendorBranchListOptions{})
	if err != nil {
		t.Fatalf("ListVendorBranches: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
}

// M55-VB6: GetVendorBranch は *ItemResult[VendorBranchEntity] を返す。
func TestGetVendorBranch_ReturnsItemResult(t *testing.T) {
	body := `{"id":42,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"name":"Branch","zip":"","pref":"","address1":"","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(body), nil
	})
	client := newVendorBranchesM55MockClient(rt)

	result, err := client.GetVendorBranch(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetVendorBranch: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ItemResult")
	}
	if result.Item == nil || result.Item.ID != 42 {
		t.Errorf("Item.ID = %d, want 42", result.Item.ID)
	}
}
