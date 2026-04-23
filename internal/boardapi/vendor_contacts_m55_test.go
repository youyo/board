package boardapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newVendorContactsM55MockClient は M55 のテスト専用モッククライアントを返す。
func newVendorContactsM55MockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// M55-VC1: buildVendorContactsQuery はゼロフィルタのとき page/per_page のみを送信する。
func TestBuildVendorContactsQuery_Zero(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("page") != "1" {
		t.Errorf("page = %q, want 1", q.Get("page"))
	}
	if q.Get("payee_id_eq") != "" {
		t.Errorf("payee_id_eq should be empty for zero filter, got %q", q.Get("payee_id_eq"))
	}
}

// M55-VC2: buildVendorContactsQuery は PayeeIDEq を payee_id_eq として送信する。
// 注意: BOARD API のリソース URL が /v1/payees であるため、Ransack FK は payee_id_eq と推定。
// E2E テストで実際のパラメータ名を確認すること。
func TestBuildVendorContactsQuery_PayeeIDEq(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{
		PayeeIDEq: 456,
	})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("payee_id_eq") != "456" {
		t.Errorf("payee_id_eq = %q, want 456", q.Get("payee_id_eq"))
	}
}

// M55-VC3: buildVendorContactsQuery は NameCont を name_cont として送信する。
func TestBuildVendorContactsQuery_NameCont(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{
		NameCont: "山田",
	})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("name_cont") != "山田" {
		t.Errorf("name_cont = %q, want 山田", q.Get("name_cont"))
	}
}

// M55-VC4: buildVendorContactsQuery は EmailCont を email_cont として送信する。
func TestBuildVendorContactsQuery_EmailCont(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{
		EmailCont: "example.com",
	})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("email_cont") != "example.com" {
		t.Errorf("email_cont = %q, want example.com", q.Get("email_cont"))
	}
}

// M55-VC5: buildVendorContactsQuery は UpdatedAtGteq と UpdatedAtLteq を送信する。
func TestBuildVendorContactsQuery_UpdatedAtRange(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
		UpdatedAtLteq: "2024-12-31 23:59:59",
	})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("updated_at_gteq") != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", q.Get("updated_at_gteq"))
	}
	if q.Get("updated_at_lteq") != "2024-12-31 23:59:59" {
		t.Errorf("updated_at_lteq = %q, want 2024-12-31 23:59:59", q.Get("updated_at_lteq"))
	}
}

// M55-VC6: buildVendorContactsQuery は IncludeArchiveFlg=false を include_archive_flg=0 として送信する。
func TestBuildVendorContactsQuery_IncludeArchiveFlgFalse(t *testing.T) {
	var gotQuery string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return jsonResp(`[]`), nil
	})
	client := newVendorContactsM55MockClient(rt)

	falseVal := false
	_, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{
		IncludeArchiveFlg: &falseVal,
	})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	q := parseQuery(t, gotQuery)
	if q.Get("include_archive_flg") != "0" {
		t.Errorf("include_archive_flg = %q, want 0", q.Get("include_archive_flg"))
	}
}

// M55-VC7: ListVendorContacts は *ListResult[VendorContactEntity] を返す。
func TestListVendorContacts_ReturnsListResult(t *testing.T) {
	page1 := `[{"id":1,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"last_name":"山田","first_name":"太郎","honorific_title":"様","title":null,"department":null,"email":null,"note":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonRespWithHeaders(page1, http.Header{"X-Total-Count": []string{"1"}}), nil
	})
	client := newVendorContactsM55MockClient(rt)

	result, err := client.ListVendorContacts(context.Background(), boardapi.VendorContactListOptions{})
	if err != nil {
		t.Fatalf("ListVendorContacts: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
}

// M55-VC8: GetVendorContact は *ItemResult[VendorContactEntity] を返す。
func TestGetVendorContact_ReturnsItemResult(t *testing.T) {
	body := `{"id":99,"vendor":{"id":10,"name":"VX","name_disp":"VX","custom_no":""},"last_name":"鈴木","first_name":"花子","honorific_title":"様","title":null,"department":null,"email":null,"note":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(body), nil
	})
	client := newVendorContactsM55MockClient(rt)

	result, err := client.GetVendorContact(context.Background(), 99)
	if err != nil {
		t.Fatalf("GetVendorContact: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ItemResult")
	}
	if result.Item == nil || result.Item.ID != 99 {
		t.Errorf("Item.ID = %d, want 99", result.Item.ID)
	}
}
