package boardapi

import (
	"net/url"
	"strings"
	"testing"
)

// TestBuildPaymentsQuery は buildPaymentsQuery が各フィールドを
// Ransack 形式のクエリ文字列に正しく変換することを検証する。
//
// 注意: payments（実 API パス /v1/expenditure_payments）は M54 時点では
// Ransack の _eq / _gteq 形式が実 API で有効かどうか未検証（E2E で確認予定）。
func TestBuildPaymentsQuery_Zero(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{}, 1, 100)
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
	if vals.Get("vendor_id_eq") != "" {
		t.Errorf("vendor_id_eq should be empty for zero filter")
	}
}

func TestBuildPaymentsQuery_VendorIDEq(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{VendorIDEq: 55}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("vendor_id_eq") != "55" {
		t.Errorf("vendor_id_eq: got %q, want %q", vals.Get("vendor_id_eq"), "55")
	}
}

func TestBuildPaymentsQuery_PurchaseOrderIDEq(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{PurchaseOrderIDEq: 123}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("purchase_order_id_eq") != "123" {
		t.Errorf("purchase_order_id_eq: got %q, want %q", vals.Get("purchase_order_id_eq"), "123")
	}
}

func TestBuildPaymentsQuery_StatusEq(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{StatusEq: "paid"}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("status_eq") != "paid" {
		t.Errorf("status_eq: got %q, want %q", vals.Get("status_eq"), "paid")
	}
}

func TestBuildPaymentsQuery_UpdatedAtRange(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{
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

func TestBuildPaymentsQuery_IncludeArchiveFlg(t *testing.T) {
	tr := true
	q := buildPaymentsQuery(PaymentListOptions{IncludeArchiveFlg: &tr}, 1, 100)
	if !strings.Contains(q, "include_archive_flg=1") {
		t.Errorf("expected include_archive_flg=1 in %q", q)
	}
}

func TestBuildPaymentsQuery_ResponseGroup(t *testing.T) {
	q := buildPaymentsQuery(PaymentListOptions{ResponseGroup: "large"}, 1, 100)
	vals, _ := url.ParseQuery(q)
	if vals.Get("response_group") != "large" {
		t.Errorf("response_group: got %q, want %q", vals.Get("response_group"), "large")
	}
}
