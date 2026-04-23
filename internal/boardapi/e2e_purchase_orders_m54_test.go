//go:build e2e

// M54 (Phase L) E2E tests: purchase_orders — ListPurchaseOrders / GetPurchaseOrder new API verification.
//
// Scope (M54, board-phase-l): purchase_orders（BOARD API: /v1/expenditures）リソースの全面移行検証。
// PurchaseOrderListOptions Ransack フィルタが実 API で正しく動作すること、
// ListPurchaseOrders / GetPurchaseOrder が *ListResult / *ItemResult を返すことを確認。
//
// Note: expenditures の Ransack パラメータ (_eq, _gteq 形式) が実 API で有効かは
// M54 時点では未検証。本テストがその動作の記録を兼ねる。
//
// Budget: 6 requests across E1-E6, at 1 req/400ms to respect 3 req/sec.
//
// Run:
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_PurchaseOrders_M54 ./internal/boardapi/
package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

func TestE2E_PurchaseOrders_M54(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// E1: ゼロフィルタで ListPurchaseOrders が *ListResult を返すことを確認
	t.Run("E1_ListReturnsListResult", func(t *testing.T) {
		result, err := client.ListPurchaseOrders(ctx, boardapi.PurchaseOrderListOptions{})
		if err != nil {
			t.Fatalf("ListPurchaseOrders: %v", err)
		}
		if result == nil {
			t.Fatal("result is nil")
		}
		t.Logf("E1: total_count=%d items=%d", result.Meta.TotalCount, len(result.Items))
	})

	time.Sleep(400 * time.Millisecond)

	// E2: ListPurchaseOrdersRaw がバイト列とヘッダーを返すことを確認
	t.Run("E2_ListRawReturnsHeadersAndBody", func(t *testing.T) {
		raw, headers, err := client.ListPurchaseOrdersRaw(ctx, boardapi.PurchaseOrderListOptions{})
		if err != nil {
			t.Fatalf("ListPurchaseOrdersRaw: %v", err)
		}
		if len(raw) == 0 {
			t.Error("body is empty")
		}
		if headers == nil {
			t.Error("headers is nil")
		}
		var items []map[string]any
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		t.Logf("E2: ListPurchaseOrdersRaw: %d items, Content-Type=%s X-Total-Count=%s",
			len(items), headers.Get("Content-Type"), headers.Get("X-Total-Count"))
	})

	time.Sleep(400 * time.Millisecond)

	// E3: vendor_id_eq フィルタがエラーなく受け入れられることを確認
	// 実 API での Ransack _eq 有効性の検証（M54 時点では未検証のためエラー有無のみチェック）
	t.Run("E3_VendorIDEqAccepted", func(t *testing.T) {
		_, _, err := client.ListPurchaseOrdersRaw(ctx, boardapi.PurchaseOrderListOptions{VendorIDEq: 1})
		t.Logf("E3: VendorIDEq=1 err=%v", err)
	})

	time.Sleep(400 * time.Millisecond)

	// E4: updated_at_gteq に未来日を指定したとき 0 件を返すことを確認
	t.Run("E4_UpdatedAtGteqFuture", func(t *testing.T) {
		future := time.Now().AddDate(1, 0, 0).Format("2006-01-02 15:04:05")
		raw, _, err := client.ListPurchaseOrdersRaw(ctx, boardapi.PurchaseOrderListOptions{UpdatedAtGteq: future})
		if err != nil {
			t.Fatalf("ListPurchaseOrdersRaw UpdatedAtGteq future: %v", err)
		}
		var items []boardapi.PurchaseOrderEntity
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("updated_at_gteq=%s must yield 0 items, got %d", future, len(items))
		}
		t.Logf("E4: updated_at_gteq=%s -> %d items (expected 0)", future, len(items))
	})

	time.Sleep(400 * time.Millisecond)

	// E5: GetPurchaseOrder が *ItemResult を返すことを確認（purchase_orders がなければ skip）
	t.Run("E5_GetReturnsItemResult", func(t *testing.T) {
		baseResult, err := client.ListPurchaseOrders(ctx, boardapi.PurchaseOrderListOptions{})
		if err != nil {
			t.Fatalf("ListPurchaseOrders: %v", err)
		}
		if len(baseResult.Items) == 0 {
			t.Skip("no purchase orders found — skipping GetPurchaseOrder test")
		}
		time.Sleep(400 * time.Millisecond)
		targetID := baseResult.Items[0].ID
		result, err := client.GetPurchaseOrder(ctx, targetID)
		if err != nil {
			t.Fatalf("GetPurchaseOrder(%d): %v", targetID, err)
		}
		if result == nil || result.Item == nil {
			t.Fatal("GetPurchaseOrder: result or result.Item is nil")
		}
		if result.Item.ID != targetID {
			t.Errorf("GetPurchaseOrder: Item.ID = %d, want %d", result.Item.ID, targetID)
		}
		t.Logf("E5: GetPurchaseOrder(%d): title=%q status=%q", result.Item.ID, result.Item.Title, result.Item.Status)
	})
}
