package boardapi_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

// TestListContacts_M52_ClientIDEq は ClientIDEq が client_id_eq として
// クエリに送信されることを確認する。
func TestListContacts_M52_ClientIDEq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newContactsMockClient(rt)

	_, err := client.ListContacts(context.Background(), boardapi.ContactListOptions{
		ClientIDEq: 456,
	})
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := gotQuery.Get("client_id_eq"); got != "456" {
		t.Errorf("client_id_eq = %q, want 456", got)
	}
}

// TestListContacts_M52_NameCont は NameCont が name_cont として
// クエリに送信されることを確認する。
func TestListContacts_M52_NameCont(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newContactsMockClient(rt)

	_, err := client.ListContacts(context.Background(), boardapi.ContactListOptions{
		NameCont: "山田",
	})
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := gotQuery.Get("name_cont"); got != "山田" {
		t.Errorf("name_cont = %q, want 山田", got)
	}
}

// TestListContacts_M52_EmailCont は EmailCont が email_cont として
// クエリに送信されることを確認する。
func TestListContacts_M52_EmailCont(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newContactsMockClient(rt)

	_, err := client.ListContacts(context.Background(), boardapi.ContactListOptions{
		EmailCont: "example.com",
	})
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := gotQuery.Get("email_cont"); got != "example.com" {
		t.Errorf("email_cont = %q, want example.com", got)
	}
}

// TestListContacts_M52_UpdatedAtGteq は UpdatedAtGteq が updated_at_gteq として
// クエリに送信されることを確認する。
func TestListContacts_M52_UpdatedAtGteq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newContactsMockClient(rt)

	_, err := client.ListContacts(context.Background(), boardapi.ContactListOptions{
		UpdatedAtGteq: "2024-06-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := gotQuery.Get("updated_at_gteq"); got != "2024-06-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-06-01 00:00:00", got)
	}
}

// TestListContacts_M52_IncludeArchiveFlg は IncludeArchiveFlg=false が
// include_archive_flg=0 として送信されることを確認する。
func TestListContacts_M52_IncludeArchiveFlg(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newContactsMockClient(rt)

	v := false
	_, err := client.ListContacts(context.Background(), boardapi.ContactListOptions{
		IncludeArchiveFlg: &v,
	})
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if got := gotQuery.Get("include_archive_flg"); got != "0" {
		t.Errorf("include_archive_flg = %q, want 0", got)
	}
}

// TestGetContact_M52_ItemResult は GetContact が *ItemResult を返すことを確認する。
func TestGetContact_M52_ItemResult(t *testing.T) {
	body := `{"id":99,"client":{"id":10,"name":"Corp A","name_disp":"Corp A","custom_no":""},"last_name":"Sato","first_name":"Hanako","honorific_title":"様","title":null,"department":null,"email":"hanako@example.com","note":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return jsonResp(body), nil
	})
	client := newContactsMockClient(rt)

	result, err := client.GetContact(context.Background(), 99)
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if gotPath != "/v1/contacts/99" {
		t.Errorf("path = %s, want /v1/contacts/99", gotPath)
	}
	if result == nil || result.Item == nil {
		t.Fatal("result or result.Item is nil")
	}
	if result.Item.ID != 99 {
		t.Errorf("item.ID = %d, want 99", result.Item.ID)
	}
}
