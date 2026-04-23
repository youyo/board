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

// newClientsMockClient returns a boardapi.Client whose HTTP client routes every
// request through rt. Mirrors newProjectCostsMockClient (M11) /
// newContactsMockClient (M10) / newClientBranchesMockClient (M09) to keep unit
// tests runnable in the sandbox where opening a local listener is denied. The
// shared roundTripperFunc and jsonResp helpers are defined in
// accounting_types_test.go (package-scope).
func newClientsMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: ListClientsRaw returns the raw JSON array body byte-for-byte when a
// single page response is served. Core ClientEntity keys must survive the
// round trip so StrictFieldDiff can later detect any unmapped BOARD API keys
// in E2E.
func TestListClientsRaw_SinglePage(t *testing.T) {
	page1 := `[{"id":1,"name":"Client A","name_disp":"A","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	var gotPath string
	var gotQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newClientsMockClient(rt)

	raw, _, err := client.ListClientsRaw(context.Background(), boardapi.ClientListOptions{})
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
	}
	if gotPath != "/v1/clients" {
		t.Errorf("path = %s, want /v1/clients", gotPath)
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
	// Core ClientEntity json tags must survive round-tripping so
	// StrictFieldDiff can detect any unmapped BOARD API keys.
	wantKeys := []string{
		"id", "name", "name_disp", "updated_at", "created_at",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key in returned raw element: %s", k)
		}
	}
}

// U2: ListClientsRaw concatenates multiple pages into a single valid JSON
// array. per_page=2 forces pagination; server returns 2 items on page 1 and
// 1 item on page 2. Result must be a JSON array of 3 items.
func TestListClientsRaw_MultiPage(t *testing.T) {
	page1Items := []string{
		`{"id":1,"name":"A","name_disp":"A","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`,
		`{"id":2,"name":"B","name_disp":"B","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-01-02T00:00:00+09:00","created_at":"2023-01-02T00:00:00+09:00"}`,
	}
	page2Items := []string{
		`{"id":3,"name":"C","name_disp":"C","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-01-03T00:00:00+09:00","created_at":"2023-01-03T00:00:00+09:00"}`,
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
	client := newClientsMockClient(rt)

	raw, _, err := client.ListClientsRaw(context.Background(), boardapi.ClientListOptions{PerPage: 2})
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
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

// U3: GetClientRaw returns body exactly as served (single object).
func TestGetClientRaw_Success(t *testing.T) {
	body := []byte(`{"id":42,"name":"Client X","name_disp":"X","title":null,"zip":null,"pref":null,"address1":null,"address2":null,"tel":null,"fax":null,"payment_term_id":100,"payment_term_name":"月末","bank_charge_to_client_flg":0,"nda_flg":0,"basic_agreement_flg":0,"document_send_type":1,"document_send_type_name":"メール","note":null,"tags":[],"company_number":null,"accounting_code":null,"to":null,"cc":null,"custom_no":null,"company_bank_id":null,"company_bank_name":null,"invoice_system_number":null,"invoice_system_number_validated":false,"invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`)
	var gotPath string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	client := newClientsMockClient(rt)

	raw, _, err := client.GetClientRaw(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetClientRaw: %v", err)
	}
	if gotPath != "/v1/clients/42" {
		t.Errorf("path = %s, want /v1/clients/42", gotPath)
	}
	if !bytes.Equal(raw, body) {
		t.Errorf("raw body mismatch\ngot:  %s\nwant: %s", string(raw), string(body))
	}
}

// U4: GetClientRaw on 404 returns *APIError{Code: APIErrorNotFound}.
func TestGetClientRaw_NotFound(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
		}, nil
	})
	client := newClientsMockClient(rt)

	_, _, err := client.GetClientRaw(context.Background(), 99)
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

// U5 (M50 刷新): ListClientsRaw sends Ransack-style `name_cont` and
// `updated_at_gteq` parameters in the query and returns the aggregated body
// plus response headers. Legacy `name` / `updated_at_from` must NOT be sent.
func TestListClientsRaw_RansackQueryParams(t *testing.T) {
	page1 := `[{"id":7,"name":"matched","name_disp":"matched","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedNameCont, observedGteq, observedLegacyName, observedLegacyFrom string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedNameCont = r.URL.Query().Get("name_cont")
		observedGteq = r.URL.Query().Get("updated_at_gteq")
		observedLegacyName = r.URL.Query().Get("name")
		observedLegacyFrom = r.URL.Query().Get("updated_at_from")
		return jsonResp(page1), nil
	})
	client := newClientsMockClient(rt)

	raw, gotHeaders, err := client.ListClientsRaw(context.Background(), boardapi.ClientListOptions{
		NameCont:      "matched",
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
	}
	if observedNameCont != "matched" {
		t.Errorf("query name_cont = %q, want matched", observedNameCont)
	}
	if observedGteq != "2024-01-01 00:00:00" {
		t.Errorf("query updated_at_gteq = %q, want 2024-01-01 00:00:00", observedGteq)
	}
	if observedLegacyName != "" || observedLegacyFrom != "" {
		t.Errorf("legacy name/updated_at_from must NOT be sent: name=%q updated_at_from=%q",
			observedLegacyName, observedLegacyFrom)
	}
	if gotHeaders == nil {
		t.Errorf("headers must be non-nil")
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("returned raw is not a valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}
}

// U5b (M50 新規): tags[] / include_archive_flg / response_group / custom_no_eq /
// invoice_system_number_eq / name_disp_cont が正しくエンコードされる。
func TestListClientsRaw_AllFilters(t *testing.T) {
	var q url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		q = r.URL.Query()
		return jsonResp("[]"), nil
	})
	client := newClientsMockClient(rt)
	trueVal := true

	_, _, err := client.ListClientsRaw(context.Background(), boardapi.ClientListOptions{
		NameDispCont:          "エス",
		InvoiceSystemNumberEq: "T1234567890123",
		CustomNoEq:            "CUST001",
		Tags:                  []string{"A", "B"},
		IncludeArchiveFlg:     &trueVal,
		ResponseGroup:         "large",
		UpdatedAtLteq:         "2024-12-31 23:59:59",
	})
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
	}
	if q.Get("name_disp_cont") != "エス" {
		t.Errorf("name_disp_cont = %q", q.Get("name_disp_cont"))
	}
	if q.Get("invoice_system_number_eq") != "T1234567890123" {
		t.Errorf("invoice_system_number_eq = %q", q.Get("invoice_system_number_eq"))
	}
	if q.Get("custom_no_eq") != "CUST001" {
		t.Errorf("custom_no_eq = %q", q.Get("custom_no_eq"))
	}
	gotTags := q["tags[]"]
	if len(gotTags) != 2 || gotTags[0] != "A" || gotTags[1] != "B" {
		t.Errorf("tags[] = %v, want [A B]", gotTags)
	}
	if q.Get("include_archive_flg") != "1" {
		t.Errorf("include_archive_flg = %q, want 1", q.Get("include_archive_flg"))
	}
	if q.Get("response_group") != "large" {
		t.Errorf("response_group = %q, want large", q.Get("response_group"))
	}
	if q.Get("updated_at_lteq") != "2024-12-31 23:59:59" {
		t.Errorf("updated_at_lteq = %q", q.Get("updated_at_lteq"))
	}
}

// ---- M43 新規 Unit テスト（U1-U6）----

// clientGetDumpJSON は実 API dump と同等の 33 フィールド JSON を返す。
const clientGetDumpJSON = `{
	"id":51285623,
	"name":"株式会社WAND",
	"name_disp":"WAND",
	"title":"御中",
	"zip":"034-0016",
	"pref":"青森県",
	"address1":"十和田市東十二番町18-31",
	"address2":null,
	"tel":null,
	"fax":null,
	"payment_term_id":53928746,
	"payment_term_name":"商品引換時",
	"bank_charge_to_client_flg":1,
	"nda_flg":1,
	"basic_agreement_flg":1,
	"document_send_type":1,
	"document_send_type_name":"メール(DL)",
	"note":null,
	"tags":[],
	"company_number":null,
	"accounting_code":null,
	"to":null,
	"cc":null,
	"custom_no":null,
	"company_bank_id":null,
	"company_bank_name":null,
	"invoice_system_number":null,
	"invoice_system_number_validated":false,
	"invoice_system_issuer_type":0,
	"invoice_system_issuer_type_name":"未設定",
	"archive_flg":0,
	"created_at":"2015-02-02T20:02:08.000+09:00",
	"updated_at":"2024-12-26T10:14:11.000+09:00"
}`

// M43 U1: Get レスポンスの全 33 フィールドが ClientEntity に正しく unmarshal される。
func TestClientEntity_UnmarshalGet_AllFields(t *testing.T) {
	var got boardapi.ClientEntity
	if err := json.Unmarshal([]byte(clientGetDumpJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 既存維持フィールド
	if got.ID != 51285623 {
		t.Errorf("ID = %d, want 51285623", got.ID)
	}
	if got.Name != "株式会社WAND" {
		t.Errorf("Name = %q, want 株式会社WAND", got.Name)
	}
	if got.CreatedAt != "2015-02-02T20:02:08.000+09:00" {
		t.Errorf("CreatedAt = %q", got.CreatedAt)
	}
	if got.UpdatedAt != "2024-12-26T10:14:11.000+09:00" {
		t.Errorf("UpdatedAt = %q", got.UpdatedAt)
	}

	// 共通フィールド
	if got.NameDisp != "WAND" {
		t.Errorf("NameDisp = %q, want WAND", got.NameDisp)
	}
	if got.Title == nil || *got.Title != "御中" {
		t.Errorf("Title = %v, want 御中", got.Title)
	}
	if got.Zip == nil || *got.Zip != "034-0016" {
		t.Errorf("Zip = %v, want 034-0016", got.Zip)
	}
	if got.Pref == nil || *got.Pref != "青森県" {
		t.Errorf("Pref = %v, want 青森県", got.Pref)
	}
	if got.Address1 == nil || *got.Address1 != "十和田市東十二番町18-31" {
		t.Errorf("Address1 = %v", got.Address1)
	}
	if got.Address2 != nil {
		t.Errorf("Address2 = %v, want nil", got.Address2)
	}
	if got.Tel != nil {
		t.Errorf("Tel = %v, want nil", got.Tel)
	}
	if got.Fax != nil {
		t.Errorf("Fax = %v, want nil", got.Fax)
	}
	if got.PaymentTermID != 53928746 {
		t.Errorf("PaymentTermID = %d, want 53928746", got.PaymentTermID)
	}
	if got.PaymentTermName != "商品引換時" {
		t.Errorf("PaymentTermName = %q, want 商品引換時", got.PaymentTermName)
	}
	if got.InvoiceSystemIssuerType != 0 {
		t.Errorf("InvoiceSystemIssuerType = %d, want 0", got.InvoiceSystemIssuerType)
	}
	if got.InvoiceSystemIssuerTypeName != "未設定" {
		t.Errorf("InvoiceSystemIssuerTypeName = %q, want 未設定", got.InvoiceSystemIssuerTypeName)
	}
	if got.InvoiceSystemNumberValidated != false {
		t.Errorf("InvoiceSystemNumberValidated = %v, want false", got.InvoiceSystemNumberValidated)
	}

	// Get 限定フィールド
	if got.BankChargeToClientFlg != 1 {
		t.Errorf("BankChargeToClientFlg = %d, want 1", got.BankChargeToClientFlg)
	}
	if got.NdaFlg != 1 {
		t.Errorf("NdaFlg = %d, want 1", got.NdaFlg)
	}
	if got.BasicAgreementFlg != 1 {
		t.Errorf("BasicAgreementFlg = %d, want 1", got.BasicAgreementFlg)
	}
	if got.DocumentSendType != 1 {
		t.Errorf("DocumentSendType = %d, want 1", got.DocumentSendType)
	}
	if got.DocumentSendTypeName != "メール(DL)" {
		t.Errorf("DocumentSendTypeName = %q, want メール(DL)", got.DocumentSendTypeName)
	}
	if got.Note != nil {
		t.Errorf("Note = %v, want nil", got.Note)
	}
	if got.Tags == nil {
		t.Errorf("Tags should not be nil (expected empty slice)")
	}
	if len(got.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", got.Tags)
	}
	if got.CompanyNumber != nil {
		t.Errorf("CompanyNumber = %v, want nil", got.CompanyNumber)
	}
	if got.AccountingCode != nil {
		t.Errorf("AccountingCode = %v, want nil", got.AccountingCode)
	}
	if got.To != nil {
		t.Errorf("To = %v, want nil", got.To)
	}
	if got.CC != nil {
		t.Errorf("CC = %v, want nil", got.CC)
	}
	if got.CustomNo != nil {
		t.Errorf("CustomNo = %v, want nil", got.CustomNo)
	}
	if got.CompanyBankID != nil {
		t.Errorf("CompanyBankID = %v, want nil", got.CompanyBankID)
	}
	if got.CompanyBankName != nil {
		t.Errorf("CompanyBankName = %v, want nil", got.CompanyBankName)
	}
	if got.InvoiceSystemNumber != nil {
		t.Errorf("InvoiceSystemNumber = %v, want nil", got.InvoiceSystemNumber)
	}
	if got.ArchiveFlg != 0 {
		t.Errorf("ArchiveFlg = %d, want 0", got.ArchiveFlg)
	}
}

// M43 U2: List レスポンス（Get 限定 14 フィールドなし）の unmarshal で
// 共通フィールドが埋まり、Get 限定フィールドは zero/nil になる。
func TestClientEntity_UnmarshalList_SparseFields(t *testing.T) {
	listJSON := `{
		"id":1,
		"name":"テスト会社",
		"name_disp":"テスト",
		"title":null,
		"zip":"100-0001",
		"pref":"東京都",
		"address1":"千代田区1-1",
		"address2":null,
		"tel":null,
		"fax":null,
		"payment_term_id":100,
		"payment_term_name":"月末",
		"company_number":null,
		"invoice_system_number":null,
		"invoice_system_number_validated":false,
		"invoice_system_issuer_type":0,
		"invoice_system_issuer_type_name":"未設定",
		"updated_at":"2024-01-01T00:00:00+09:00",
		"created_at":"2023-01-01T00:00:00+09:00"
	}`
	var got boardapi.ClientEntity
	if err := json.Unmarshal([]byte(listJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 共通フィールドが埋まる
	if got.ID != 1 {
		t.Errorf("ID = %d, want 1", got.ID)
	}
	if got.Name != "テスト会社" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.NameDisp != "テスト" {
		t.Errorf("NameDisp = %q", got.NameDisp)
	}
	if got.PaymentTermID != 100 {
		t.Errorf("PaymentTermID = %d, want 100", got.PaymentTermID)
	}
	// Get 限定フィールドは zero/nil
	if got.AccountingCode != nil {
		t.Errorf("AccountingCode should be nil for List response")
	}
	if got.Note != nil {
		t.Errorf("Note should be nil for List response")
	}
	if got.Tags != nil {
		t.Errorf("Tags should be nil for List response (omitempty)")
	}
	if got.DocumentSendTypeName != "" {
		t.Errorf("DocumentSendTypeName should be empty for List response")
	}
}

// M43 U3: address2=null の JSON で Address2 が nil になる。
func TestClientEntity_NullableString_NilForMissing(t *testing.T) {
	j := `{"id":1,"name":"X","name_disp":"X","payment_term_id":1,"payment_term_name":"a","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"address2":null,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2024-01-01T00:00:00+09:00"}`
	var got boardapi.ClientEntity
	if err := json.Unmarshal([]byte(j), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Address2 != nil {
		t.Errorf("Address2 = %v, want nil for null JSON value", got.Address2)
	}
}

// M43 U5 (M50 刷新): ClientListOptions が日本語 name_cont / updated_at_gteq
// クエリを URL エンコード含めて正しく送る。
func TestClientListOptions_QueryEncoding(t *testing.T) {
	page1 := `[{"id":7,"name":"matched","name_disp":"matched","payment_term_id":100,"payment_term_name":"月末","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"updated_at":"2024-02-01T00:00:00+09:00","created_at":"2024-02-01T00:00:00+09:00"}]`
	var observedQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedQuery = r.URL.Query()
		return jsonResp(page1), nil
	})
	client := newClientsMockClient(rt)

	_, _, err := client.ListClientsRaw(context.Background(), boardapi.ClientListOptions{
		NameCont:      "テスト",
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListClientsRaw: %v", err)
	}
	if observedQuery.Get("name_cont") != "テスト" {
		t.Errorf("query name_cont = %q, want テスト", observedQuery.Get("name_cont"))
	}
	if observedQuery.Get("updated_at_gteq") != "2024-01-01 00:00:00" {
		t.Errorf("query updated_at_gteq = %q", observedQuery.Get("updated_at_gteq"))
	}
}

// M43 U6: tags=[] の JSON で Tags が nil ではなく空スライスになる。
func TestClientEntity_TagsEmptyArray(t *testing.T) {
	j := `{"id":1,"name":"X","name_disp":"X","payment_term_id":1,"payment_term_name":"a","invoice_system_issuer_type":0,"invoice_system_issuer_type_name":"未設定","invoice_system_number_validated":false,"tags":[],"bank_charge_to_client_flg":0,"nda_flg":0,"basic_agreement_flg":0,"document_send_type":1,"document_send_type_name":"メール","archive_flg":0,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2024-01-01T00:00:00+09:00"}`
	var got boardapi.ClientEntity
	if err := json.Unmarshal([]byte(j), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Tags == nil {
		t.Errorf("Tags = nil, want empty slice (tags:[] should decode to []string{})")
	}
	if len(got.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", got.Tags)
	}
}
