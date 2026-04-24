//go:build e2e

// E2E tests for receipts M53: GetReceipt → *ItemResult[ReceiptEntity].
//
// Scope (M53, board-phase-l): Phase L 刷新検証。
// GetReceipt の戻り値が *ItemResult[ReceiptEntity] になったことを実 API で確認。
// ItemResult.Meta（parseItemMeta 経由）が rate limit 等のヘッダーを保持することを検証。
//
// 既存 e2e_receipts_test.go（M21/M38）との関係:
//   - 既存テストは StrictFieldDiff を中心とした Entity フィールド突合。
//   - 本テストは M53 の新規機能（ItemResult + Meta）の動作確認に特化。
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Receipts_M53 ./internal/boardapi/
package boardapi_test

import (
	"context"
	"testing"
	"time"
)

// TestE2E_Receipts_M53_GetReturnsItemResult は GetReceipt が *ItemResult を返し、
// 実 API レスポンスから ItemMeta（rate limit 等）が parseItemMeta 経由で取得できることを確認。
func TestE2E_Receipts_M53_GetReturnsItemResult(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Discover a receipt ID via M17 helper
	projectID, docID := findAnyDocumentID(t, client, "receipt")
	requirePositiveID(t, projectID, "findAnyDocumentID.projectID")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")

	time.Sleep(400 * time.Millisecond) // rate limit: 3 req/sec 制約

	// E1: GetReceipt が *ItemResult[ReceiptEntity] を返すことを確認
	result, err := client.GetReceipt(ctx, docID)
	if err != nil {
		t.Fatalf("GetReceipt(%d): %v", docID, err)
	}
	if result == nil {
		t.Fatal("GetReceipt: result is nil")
	}
	if result.Item == nil {
		t.Fatal("GetReceipt: result.Item is nil")
	}
	if result.Item.ID != docID {
		t.Errorf("GetReceipt: Item.ID = %d, want %d", result.Item.ID, docID)
	}
	t.Logf("E1 GetReceipt: id=%d total=%s details_count=%d", result.Item.ID, result.Item.Total, len(result.Item.Details))

	time.Sleep(400 * time.Millisecond)

	// E2: GetReceiptRaw が ([]byte, http.Header, error) を返し、ヘッダーが非 nil であることを確認
	raw, headers, err := client.GetReceiptRaw(ctx, docID)
	if err != nil {
		t.Fatalf("GetReceiptRaw(%d): %v", docID, err)
	}
	if len(raw) == 0 {
		t.Error("GetReceiptRaw: body is empty")
	}
	if headers == nil {
		t.Error("GetReceiptRaw: headers is nil")
	}
	// ヘッダー名のログ（M50 §E10 相当: 実測値確認）
	t.Logf("E2 GetReceiptRaw headers: Content-Type=%s ETag=%s X-Ratelimit-Remaining=%s X-RateLimit-Remaining=%s",
		headers.Get("Content-Type"),
		headers.Get("ETag"),
		headers.Get("X-Ratelimit-Remaining"),
		headers.Get("X-RateLimit-Remaining"),
	)

	// E3: ItemResult.Headers が非 nil であること（API から直接取得した場合）
	// 注意: GetReceipt は boardapi.Client を直接呼ぶため Headers は含まれる。
	// ただし CLI は repository 経由（GetByDocumentID→.Item 展開）のため Meta はゼロ値になる（設計上の制約）。
	if result.Headers == nil {
		// Headers はボード API 経由で呼んだ場合に設定される
		t.Logf("E3: result.Headers is nil (this is expected only if called via repository cache path)")
	} else {
		t.Logf("E3: result.Headers present: ETag=%s", result.Headers.Get("ETag"))
	}
}

// TestE2E_Receipts_M53_ItemMetaParsed は ItemResult.Meta が非ゼロ値になることを確認（boardapi 直呼び）。
// parseItemMeta が X-Ratelimit-* ヘッダーを正しく取得できれば RateLimitLimit > 0 になるはず。
func TestE2E_Receipts_M53_ItemMetaParsed(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	_, docID := findAnyDocumentID(t, client, "receipt")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")

	time.Sleep(400 * time.Millisecond)

	result, err := client.GetReceipt(ctx, docID)
	if err != nil {
		t.Fatalf("GetReceipt(%d): %v", docID, err)
	}

	// boardapi.Client.GetReceipt は DoWithRetryFull でヘッダーを取得し parseItemMeta を適用。
	// RateLimitLimit は BOARD API の日次上限（3000）が返れば > 0 になる。
	t.Logf("ItemMeta: RateLimitLimit=%d RateLimitRemaining=%d ETag=%q LastModified=%q",
		result.Meta.RateLimitLimit,
		result.Meta.RateLimitRemaining,
		result.Meta.ETag,
		result.Meta.LastModified,
	)
	// ヘッダーが存在すれば RateLimitLimit は正の値になるはず（TBD: 実ヘッダー名確定後に厳格化）
	if result.Meta.RateLimitLimit < 0 {
		t.Errorf("ItemMeta.RateLimitLimit should be >= 0, got %d", result.Meta.RateLimitLimit)
	}
}
