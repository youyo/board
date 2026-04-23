package boardapi_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

// TestListProjectCosts_M52_ProjectIDEq は ProjectIDEq が project_id_eq として
// クエリに送信されることを確認する。
func TestListProjectCosts_M52_ProjectIDEq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.ListProjectCosts(context.Background(), boardapi.ProjectCostListOptions{
		ProjectIDEq: 85079735,
	})
	if err != nil {
		t.Fatalf("ListProjectCosts: %v", err)
	}
	if got := gotQuery.Get("project_id_eq"); got != "85079735" {
		t.Errorf("project_id_eq = %q, want 85079735", got)
	}
}

// TestListProjectCosts_M52_UpdatedAtGteq は UpdatedAtGteq が
// updated_at_gteq としてクエリに送信されることを確認する。
func TestListProjectCosts_M52_UpdatedAtGteq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.ListProjectCosts(context.Background(), boardapi.ProjectCostListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListProjectCosts: %v", err)
	}
	if got := gotQuery.Get("updated_at_gteq"); got != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", got)
	}
}

// TestListProjectCosts_M52_UpdatedAtLteq は UpdatedAtLteq が
// updated_at_lteq としてクエリに送信されることを確認する。
func TestListProjectCosts_M52_UpdatedAtLteq(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.ListProjectCosts(context.Background(), boardapi.ProjectCostListOptions{
		UpdatedAtLteq: "2024-12-31 23:59:59",
	})
	if err != nil {
		t.Fatalf("ListProjectCosts: %v", err)
	}
	if got := gotQuery.Get("updated_at_lteq"); got != "2024-12-31 23:59:59" {
		t.Errorf("updated_at_lteq = %q, want 2024-12-31 23:59:59", got)
	}
}

// TestListProjectCosts_M52_IncludeArchiveFlg は IncludeArchiveFlg が
// include_archive_flg=1 として送信されることを確認する。
func TestListProjectCosts_M52_IncludeArchiveFlg(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newProjectCostsMockClient(rt)

	v := true
	_, err := client.ListProjectCosts(context.Background(), boardapi.ProjectCostListOptions{
		IncludeArchiveFlg: &v,
	})
	if err != nil {
		t.Fatalf("ListProjectCosts: %v", err)
	}
	if got := gotQuery.Get("include_archive_flg"); got != "1" {
		t.Errorf("include_archive_flg = %q, want 1", got)
	}
}

// TestListProjectCosts_M52_ZeroOptions はゼロオプション時に余分なパラメータが
// 送信されないことを確認する。
func TestListProjectCosts_M52_ZeroOptions(t *testing.T) {
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newProjectCostsMockClient(rt)

	_, err := client.ListProjectCosts(context.Background(), boardapi.ProjectCostListOptions{})
	if err != nil {
		t.Fatalf("ListProjectCosts: %v", err)
	}
	for _, key := range []string{"project_id_eq", "updated_at_gteq", "updated_at_lteq", "include_archive_flg"} {
		if got := gotQuery.Get(key); got != "" {
			t.Errorf("unexpected query param %s = %q", key, got)
		}
	}
}

// TestGetProjectCost_M52_ItemResult は GetProjectCost が *ItemResult を返すことを確認する。
func TestGetProjectCost_M52_ItemResult(t *testing.T) {
	body := `{"id":33291004,"project_id":85079735,"description":"S2への支払い","cost":400000,"invoice_date":"2020-02-01","payment_date":"2020-02-29","created_at":"2020-06-15T11:02:58.000+09:00","updated_at":"2020-06-15T11:02:58.000+09:00"}`
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return jsonResp(body), nil
	})
	client := newProjectCostsMockClient(rt)

	result, err := client.GetProjectCost(context.Background(), 33291004)
	if err != nil {
		t.Fatalf("GetProjectCost: %v", err)
	}
	if gotPath != "/v1/project_costs/33291004" {
		t.Errorf("path = %s, want /v1/project_costs/33291004", gotPath)
	}
	if result == nil || result.Item == nil {
		t.Fatal("result or result.Item is nil")
	}
	if result.Item.ID != 33291004 {
		t.Errorf("item.ID = %d, want 33291004", result.Item.ID)
	}
}
