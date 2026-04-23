package boardapi

import (
	"net/url"
	"strings"
	"testing"
)

// TestBuildInvoicesQuery は buildInvoicesQuery が各フィールドを
// Ransack 形式のクエリ文字列に正しく変換することを検証する。
//
// 注意: invoices/expenditures/expenditure_payments は M54 時点では
// Ransack の _eq / _gteq 形式が実 API で有効かどうか未検証（E2E で確認予定）。
func TestBuildInvoicesQuery_Zero(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{}, 1, 100)
	vals, err := url.ParseQuery(q)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if vals.Get("page") != "1" {
		t.Errorf("page: got %q, want %q", vals.Get("page"), "1")
	}
	if vals.Get("per_page") != "100" {
		t.Errorf("per_page: got %q, want %q", vals.Get("per_page"), "100")
	}
	// ゼロ値は送信しない
	if vals.Get("client_id_eq") != "" {
		t.Errorf("client_id_eq should be empty for zero filter")
	}
	if vals.Get("status_eq") != "" {
		t.Errorf("status_eq should be empty for zero filter")
	}
}

func TestBuildInvoicesQuery_ClientIDEq(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{ClientIDEq: 42}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("client_id_eq") != "42" {
		t.Errorf("client_id_eq: got %q, want %q", vals.Get("client_id_eq"), "42")
	}
}

func TestBuildInvoicesQuery_ProjectIDEq(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{ProjectIDEq: 7}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("project_id_eq") != "7" {
		t.Errorf("project_id_eq: got %q, want %q", vals.Get("project_id_eq"), "7")
	}
}

func TestBuildInvoicesQuery_StatusEq(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{StatusEq: "complete"}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("status_eq") != "complete" {
		t.Errorf("status_eq: got %q, want %q", vals.Get("status_eq"), "complete")
	}
}

func TestBuildInvoicesQuery_UpdatedAtRange(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
		UpdatedAtLteq: "2024-12-31 23:59:59",
	}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("updated_at_gteq") != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq: got %q", vals.Get("updated_at_gteq"))
	}
	if vals.Get("updated_at_lteq") != "2024-12-31 23:59:59" {
		t.Errorf("updated_at_lteq: got %q", vals.Get("updated_at_lteq"))
	}
}

func TestBuildInvoicesQuery_IncludeArchiveFlg(t *testing.T) {
	tr := true
	q := buildInvoicesQuery(InvoiceListOptions{IncludeArchiveFlg: &tr}, 1, 100)
	if !strings.Contains(q, "include_archive_flg=1") {
		t.Errorf("expected include_archive_flg=1 in %q", q)
	}
}

func TestBuildInvoicesQuery_ResponseGroup(t *testing.T) {
	q := buildInvoicesQuery(InvoiceListOptions{ResponseGroup: "large"}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("response_group") != "large" {
		t.Errorf("response_group: got %q, want %q", vals.Get("response_group"), "large")
	}
}
