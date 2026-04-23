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

// newEstimatesMockClient returns a boardapi.Client with a mock HTTP transport.
// Mirrors newClientsMockClient pattern (M50).
func newEstimatesMockClient(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// U1: TestGetEstimate_ReturnsItemResult — GetEstimate が *ItemResult[EstimateEntity] を
// 返し、Item.ID が正しく設定されていることを確認。
func TestGetEstimate_ReturnsItemResult(t *testing.T) {
	raw := `{"id":1001,"message":"備考","total":"100000","tax":"10000","tax_withholding":"0","seal_approval_status":0,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[],"valid_period":"2週間"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/documents/estimates/1001" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(raw)),
		}, nil
	})
	client := newEstimatesMockClient(rt)

	result, err := client.GetEstimate(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetEstimate: %v", err)
	}
	if result == nil {
		t.Fatal("GetEstimate: result is nil")
	}
	if result.Item == nil {
		t.Fatal("GetEstimate: result.Item is nil")
	}
	if result.Item.ID != 1001 {
		t.Errorf("Item.ID = %d, want 1001", result.Item.ID)
	}
	if result.Item.Total != "100000" {
		t.Errorf("Item.Total = %s, want 100000", result.Item.Total)
	}
}

// U2: TestGetEstimateRaw_ReturnsBodyAndHeader — GetEstimateRaw が ([]byte, http.Header, error) を
// 返すことを確認（旧: ([]byte, error) → 新: ([]byte, http.Header, error)）。
func TestGetEstimateRaw_ReturnsBodyAndHeader(t *testing.T) {
	rawBody := `{"id":1002,"total":"50000","tax":"5000","tax_withholding":"0","seal_approval_status":0,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"details":[],"valid_period":""}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Etag":         []string{`"abc123"`},
			},
			Body: io.NopCloser(bytes.NewBufferString(rawBody)),
		}, nil
	})
	client := newEstimatesMockClient(rt)

	body, headers, err := client.GetEstimateRaw(context.Background(), 1002)
	if err != nil {
		t.Fatalf("GetEstimateRaw: %v", err)
	}
	if string(body) != rawBody {
		t.Errorf("body mismatch: got %s", string(body))
	}
	if headers == nil {
		t.Error("headers should not be nil")
	}
	if headers.Get("Etag") == "" {
		t.Error("Etag header should be present")
	}
}

// U3: TestGetOrder_ReturnsItemResult — GetOrder が *ItemResult[OrderEntity] を返すことを確認。
func TestGetOrder_ReturnsItemResult(t *testing.T) {
	raw := `{"id":2001,"message":null,"total":"200000","tax":"20000","tax_withholding":"0","seal_approval_status":0,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[],"disp_order_date":null,"disp_order_receive_date":null}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/documents/orders/2001" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(raw)),
		}, nil
	})
	client := newEstimatesMockClient(rt)

	result, err := client.GetOrder(context.Background(), 2001)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if result == nil || result.Item == nil {
		t.Fatal("GetOrder: result or result.Item is nil")
	}
	if result.Item.ID != 2001 {
		t.Errorf("Item.ID = %d, want 2001", result.Item.ID)
	}
}

// U4: TestGetDelivery_ReturnsItemResult — GetDelivery が *ItemResult[DeliveryEntity] を返すことを確認。
func TestGetDelivery_ReturnsItemResult(t *testing.T) {
	raw := `{"id":3001,"message":null,"total":"300000","tax":"30000","tax_withholding":"0","seal_approval_status":0,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[],"delivery_date":"2024-03-01","disp_delivery_date":null,"disp_delivery_receive_date":null}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/documents/deliveries/3001" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(raw)),
		}, nil
	})
	client := newEstimatesMockClient(rt)

	result, err := client.GetDelivery(context.Background(), 3001)
	if err != nil {
		t.Fatalf("GetDelivery: %v", err)
	}
	if result == nil || result.Item == nil {
		t.Fatal("GetDelivery: result or result.Item is nil")
	}
	if result.Item.ID != 3001 {
		t.Errorf("Item.ID = %d, want 3001", result.Item.ID)
	}
}

// U5: TestGetReceipt_ReturnsItemResult — GetReceipt が *ItemResult[ReceiptEntity] を返すことを確認。
func TestGetReceipt_ReturnsItemResult(t *testing.T) {
	raw := `{"id":4001,"message":null,"total":"400000","tax":"40000","tax_withholding":"0","seal_approval_status":0,"document_amount_disp_kbn":1,"blank_date_flg":0,"lock_flg":0,"delivery_place":null,"details":[],"receipt_date":"2024-04-01","disp_receipt_date":null}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/documents/receipts/4001" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(raw)),
		}, nil
	})
	client := newEstimatesMockClient(rt)

	result, err := client.GetReceipt(context.Background(), 4001)
	if err != nil {
		t.Fatalf("GetReceipt: %v", err)
	}
	if result == nil || result.Item == nil {
		t.Fatal("GetReceipt: result or result.Item is nil")
	}
	if result.Item.ID != 4001 {
		t.Errorf("Item.ID = %d, want 4001", result.Item.ID)
	}
}
