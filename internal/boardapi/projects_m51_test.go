package boardapi_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

func newProjectsM51MockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// ===== buildProjectsQuery TDD テスト (Red → Green) =====

// T1: ゼロ値 ProjectListOptions は page/per_page のみ（ページネーション自動設定を確認）
func TestBuildProjectsQuery_Zero(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		// ゼロ値フィルタはクエリに余分なパラメータを追加しない
		if q.Get("name_cont") != "" {
			t.Errorf("name_cont should be empty, got %q", q.Get("name_cont"))
		}
		if q.Get("client_id_eq") != "" {
			t.Errorf("client_id_eq should be empty, got %q", q.Get("client_id_eq"))
		}
		if q.Get("response_group") != "" {
			t.Errorf("response_group should be empty, got %q", q.Get("response_group"))
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T2: NameCont → name_cont=<value>
func TestBuildProjectsQuery_NameCont(t *testing.T) {
	const page1 = `[{"id":1,"name":"テスト","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("name_cont"); got != "テスト" {
			t.Errorf("name_cont = %q, want テスト", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{NameCont: "テスト"})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T3: ClientIDEq → client_id_eq=<value>
func TestBuildProjectsQuery_ClientIDEq(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("client_id_eq"); got != "12345" {
			t.Errorf("client_id_eq = %q, want 12345", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{ClientIDEq: 12345})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T4: OrderStatusIn → order_status_in[]=1&order_status_in[]=2
func TestBuildProjectsQuery_OrderStatusIn(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		vals := r.URL.Query()["order_status_in[]"]
		if len(vals) != 2 {
			t.Errorf("order_status_in[] count = %d, want 2; vals=%v", len(vals), vals)
		} else {
			if vals[0] != "1" || vals[1] != "2" {
				t.Errorf("order_status_in[] = %v, want [1 2]", vals)
			}
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{OrderStatusIn: []int{1, 2}})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T5: DeliveryStatusIn → delivery_status_in[]=<value>
func TestBuildProjectsQuery_DeliveryStatusIn(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		vals := r.URL.Query()["delivery_status_in[]"]
		if len(vals) != 1 || vals[0] != "3" {
			t.Errorf("delivery_status_in[] = %v, want [3]", vals)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{DeliveryStatusIn: []int{3}})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T6: IncludeLostFlg=true → include_lost_flg=1
func TestBuildProjectsQuery_IncludeLostFlg_True(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("include_lost_flg"); got != "1" {
			t.Errorf("include_lost_flg = %q, want 1", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	v := true
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{IncludeLostFlg: &v})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T7: IncludeLostFlg=nil → include_lost_flg は送らない
func TestBuildProjectsQuery_IncludeLostFlg_Nil(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if _, ok := r.URL.Query()["include_lost_flg"]; ok {
			t.Error("include_lost_flg should not be present when nil")
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{IncludeLostFlg: nil})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T8: IncludeArchiveFlg=false → include_archive_flg=0
func TestBuildProjectsQuery_IncludeArchiveFlg_False(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("include_archive_flg"); got != "0" {
			t.Errorf("include_archive_flg = %q, want 0", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	v := false
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{IncludeArchiveFlg: &v})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T9: ResponseGroup="large" → response_group=large
func TestBuildProjectsQuery_ResponseGroup(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("response_group"); got != "large" {
			t.Errorf("response_group = %q, want large", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{ResponseGroup: "large"})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T10: UpdatedAtGteq / UpdatedAtLteq → updated_at_gteq / updated_at_lteq
func TestBuildProjectsQuery_UpdatedAtRange(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("updated_at_gteq"); got != "2024-01-01 00:00:00" {
			t.Errorf("updated_at_gteq = %q, want 2024-01-01 00:00:00", got)
		}
		if got := r.URL.Query().Get("updated_at_lteq"); got != "2024-12-31 23:59:59" {
			t.Errorf("updated_at_lteq = %q, want 2024-12-31 23:59:59", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
		UpdatedAtLteq: "2024-12-31 23:59:59",
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T11: Tags → tags[]=A&tags[]=B
func TestBuildProjectsQuery_Tags(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		tags := r.URL.Query()["tags[]"]
		if len(tags) != 2 {
			t.Errorf("tags[] count = %d, want 2; tags=%v", len(tags), tags)
		} else {
			if tags[0] != "tagA" || tags[1] != "tagB" {
				t.Errorf("tags[] = %v, want [tagA tagB]", tags)
			}
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{Tags: []string{"tagA", "tagB"}})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T12: ProjectNoEq → project_no_eq=<value>
func TestBuildProjectsQuery_ProjectNoEq(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("project_no_eq"); got != "P-001" {
			t.Errorf("project_no_eq = %q, want P-001", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{ProjectNoEq: "P-001"})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// T13: ListProjects returns *ListResult with Items populated
func TestListProjects_ReturnsListResult(t *testing.T) {
	const page1 = `[{"id":42,"name":"TestProject","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	result, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ID != 42 {
		t.Errorf("Items[0].ID = %d, want 42", result.Items[0].ID)
	}
}

// T14: GetProject returns *ItemResult with Item populated
func TestGetProject_ReturnsItemResult(t *testing.T) {
	const body = `{"id":99,"name":"SingleProject","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/projects/99" {
			t.Errorf("path = %s, want /v1/projects/99", r.URL.Path)
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		}, nil
	})
	client := newProjectsM51MockClient(rt)
	result, err := client.GetProject(context.Background(), 99)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if result == nil || result.Item == nil {
		t.Fatal("result.Item should not be nil")
	}
	if result.Item.ID != 99 {
		t.Errorf("Item.ID = %d, want 99", result.Item.ID)
	}
}

// T15: GetProjectWithGroup (M51) sends response_group as query param (QueryBuilder経由)
// and returns *ItemResult[ProjectEntity]
func TestGetProjectWithGroup_M51_ReturnsItemResult(t *testing.T) {
	const body = `{"id":42,"name":"P","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"","created_at":""}`
	for _, group := range []string{"estimate", "order", "delivery", "invoice", "receipt", "all"} {
		t.Run(group, func(t *testing.T) {
			rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if got := r.URL.Query().Get("response_group"); got != group {
					t.Errorf("response_group = %q, want %q", got, group)
				}
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader([]byte(body))),
				}, nil
			})
			client := newProjectsM51MockClient(rt)
			result, err := client.GetProjectWithGroup(context.Background(), 42, group)
			if err != nil {
				t.Fatalf("GetProjectWithGroup(%s): %v", group, err)
			}
			if result == nil || result.Item == nil {
				t.Fatalf("result.Item should not be nil for group %s", group)
			}
		})
	}
}

// T16: DeliveryDateGteq / InvoiceDateLteq → delivery_date_gteq / invoice_date_lteq
func TestBuildProjectsQuery_DateFilters(t *testing.T) {
	const page1 = `[{"id":1,"name":"A","order_status":1,"order_status_name":"受注","delivery_status":1,"delivery_status_name":"未着手","invoice_dates":[],"tags":[],"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("delivery_date_gteq"); got != "2024-01-01" {
			t.Errorf("delivery_date_gteq = %q, want 2024-01-01", got)
		}
		if got := r.URL.Query().Get("delivery_date_lteq"); got != "2024-12-31" {
			t.Errorf("delivery_date_lteq = %q, want 2024-12-31", got)
		}
		if got := r.URL.Query().Get("invoice_date_gteq"); got != "2024-02-01" {
			t.Errorf("invoice_date_gteq = %q, want 2024-02-01", got)
		}
		return jsonResp(page1), nil
	})
	client := newProjectsM51MockClient(rt)
	_, err := client.ListProjects(context.Background(), boardapi.ProjectListOptions{
		DeliveryDateGteq: "2024-01-01",
		DeliveryDateLteq: "2024-12-31",
		InvoiceDateGteq:  "2024-02-01",
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}
