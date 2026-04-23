//go:build e2e

// E2E tests for /v1/documents/receipts against the real BOARD API.
//
// Scope (M21/M38, board-compliance roadmap): Phase G (document) 5th milestone.
// receipts は document 系エンドポイントで Get のみ提供（List/Search は対象外）。
// documentID discovery は M17 で確立した findAnyDocumentID helper を経由する。
//
//   - Get (TestE2E_Receipts_Get) : ~3 req expected
//     (ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetReceiptRaw 1)
//
// Total budget: ~10 req (plan cap 15).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when findAnyDocumentID cannot find any receipt in the
// top maxDiscoveryProjects projects, the test skips with "pending
// re-verification" — tracked in the roadmap and re-run once data is seeded.
//
// PII handling: raw artifacts are written under tmp/e2e-artifacts/ (gitignored)
// for manual inspection. t.Logf output avoids leaking PII (message text);
// only lengths, ids, and numeric fields are logged.
//
// Usage (single-shot):
//
//	BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 \
//	    -run TestE2E_Receipts_Get ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Receipts_Get discovers a receipt document ID via M17 helper,
// fetches the detail via GetReceiptRaw, and applies strict field diff on the
// single-object response.
//
// Discovery is delegated to findAnyDocumentID(t, client, "receipt"), which
// scans the top maxDiscoveryProjects projects with response_group=receipt.
// Data-dependent skip is handled inside the helper.
// A 403 or 429 on the discovered ID surfaces via t.Fatalf (not skipped),
// per the BOARD compliance roadmap rule: no skip on 403/429.
func TestE2E_Receipts_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Step 1: M17 helper で receipt の projectID と documentID を取得
	projectID, docID := findAnyDocumentID(t, client, "receipt")
	requirePositiveID(t, projectID, "findAnyDocumentID.projectID")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")

	// Step 2: GetReceiptRaw で生レスポンスを取得
	getRaw, _, err := client.GetReceiptRaw(ctx, docID)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("GetReceiptRaw(%d): %v", docID, err)
	}

	// Step 3: 生 JSON をアーティファクトとして保存（best-effort）
	dumpJSON(t, "receipts", docID, getRaw)

	// Step 4: 厳格フィールド突合 — ReceiptEntity に未マップフィールドがあれば失敗
	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.ReceiptEntity{}); len(diff) > 0 {
		t.Errorf("receipts get(%d) unmapped fields: %v", docID, diff)
	}

	// Step 5: エンティティに Unmarshal して基本フィールド検証
	var got boardapi.ReceiptEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != docID {
		t.Errorf("GetReceipt ID mismatch: got=%d want=%d", got.ID, docID)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	var msgLen int
	if got.Message != nil {
		msgLen = len(*got.Message)
	}
	t.Logf("TestE2E_Receipts_Get: id=%d message_len=%d total=%s tax=%s details_count=%d receipt_date=%s",
		got.ID, msgLen, got.Total, got.Tax, len(got.Details), got.ReceiptDate)
}
