package boardapi_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

// TestBuildClientBranchesQuery_* は boardapi.buildClientBranchesQuery の
// URL クエリ組み立てを検証するユニットテスト（M52 Red フェーズ）。
// 実際のクエリは ListClientBranches 経由のリクエストで観測する。

// TestListClientBranches_M52_ClientIDEq は ClientIDEq が client_id_eq として
// クエリに送信されることを確認する。
func TestListClientBranches_M52_ClientIDEq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newClientBranchesMockClient(rt)

	_, err := client.ListClientBranches(context.Background(), boardapi.ClientBranchListOptions{
		ClientIDEq: 123,
	})
	if err != nil {
		t.Fatalf("ListClientBranches: %v", err)
	}
	if got := gotQuery.Get("client_id_eq"); got != "123" {
		t.Errorf("client_id_eq = %q, want 123", got)
	}
}

// TestListClientBranches_M52_NameCont は NameCont が name_cont として
// クエリに送信されることを確認する。
func TestListClientBranches_M52_NameCont(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newClientBranchesMockClient(rt)

	_, err := client.ListClientBranches(context.Background(), boardapi.ClientBranchListOptions{
		NameCont: "keyword",
	})
	if err != nil {
		t.Fatalf("ListClientBranches: %v", err)
	}
	if got := gotQuery.Get("name_cont"); got != "keyword" {
		t.Errorf("name_cont = %q, want keyword", got)
	}
}

// TestListClientBranches_M52_UpdatedAtGteq は UpdatedAtGteq が
// updated_at_gteq としてクエリに送信されることを確認する。
func TestListClientBranches_M52_UpdatedAtGteq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newClientBranchesMockClient(rt)

	_, err := client.ListClientBranches(context.Background(), boardapi.ClientBranchListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListClientBranches: %v", err)
	}
	if got := gotQuery.Get("updated_at_gteq"); got != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", got)
	}
}

// TestListClientBranches_M52_IncludeArchiveFlg は IncludeArchiveFlg が
// include_archive_flg=1 として送信されることを確認する。
func TestListClientBranches_M52_IncludeArchiveFlg(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newClientBranchesMockClient(rt)

	v := true
	_, err := client.ListClientBranches(context.Background(), boardapi.ClientBranchListOptions{
		IncludeArchiveFlg: &v,
	})
	if err != nil {
		t.Fatalf("ListClientBranches: %v", err)
	}
	if got := gotQuery.Get("include_archive_flg"); got != "1" {
		t.Errorf("include_archive_flg = %q, want 1", got)
	}
}

// TestListClientBranches_M52_ZeroOptions はゼロオプション時に余分なパラメータが
// 送信されないことを確認する。
func TestListClientBranches_M52_ZeroOptions(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newClientBranchesMockClient(rt)

	_, err := client.ListClientBranches(context.Background(), boardapi.ClientBranchListOptions{})
	if err != nil {
		t.Fatalf("ListClientBranches: %v", err)
	}
	for _, key := range []string{"client_id_eq", "name_cont", "updated_at_gteq", "updated_at_lteq", "include_archive_flg"} {
		if got := gotQuery.Get(key); got != "" {
			t.Errorf("unexpected query param %s = %q", key, got)
		}
	}
}

// TestGetClientBranch_M52_ItemResult は GetClientBranch が *ItemResult を返すことを確認する。
func TestGetClientBranch_M52_ItemResult(t *testing.T) {
	body := `{"id":42,"client":{"id":10,"name":"テスト","name_disp":"テスト","custom_no":""},"name":"本社","zip":"100-0001","pref":"東京都","address1":"千代田区1-1","address2":"","tel":null,"fax":null,"archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return jsonResp(body), nil
	})
	client := newClientBranchesMockClient(rt)

	result, err := client.GetClientBranch(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetClientBranch: %v", err)
	}
	if gotPath != "/v1/client_branches/42" {
		t.Errorf("path = %s, want /v1/client_branches/42", gotPath)
	}
	if result == nil || result.Item == nil {
		t.Fatal("result or result.Item is nil")
	}
	if result.Item.ID != 42 {
		t.Errorf("item.ID = %d, want 42", result.Item.ID)
	}
}
