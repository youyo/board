//go:build e2e

// E2E tests for /v1/documents/deliveries against the real BOARD API.
//
// Scope (M20/M37, board-compliance roadmap): Phase G (document) 4th milestone.
// deliveries は document 系エンドポイントで Get のみ提供（List/Search は対象外）。
// documentID discovery は M17 で確立した findAnyDocumentID helper を経由する。
//
//   - Get (TestE2E_Deliveries_Get) : ~3 req expected
//     (ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetDeliveryRaw 1)
//
// Total budget: ~10 req (plan cap 15).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when findAnyDocumentID cannot find any delivery in the
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
//	    -run TestE2E_Deliveries_Get ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Deliveries_Get discovers a delivery document ID via M17 helper,
// fetches the detail via GetDeliveryRaw, and applies strict field diff on the
// single-object response.
//
// Discovery is delegated to findAnyDocumentID(t, client, "delivery"), which
// scans the top maxDiscoveryProjects projects with response_group=delivery.
// Data-dependent skip is handled inside the helper.
// A 403 or 429 on the discovered ID surfaces via t.Fatalf (not skipped),
// per the BOARD compliance roadmap rule: no skip on 403/429.
func TestE2E_Deliveries_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Step 1: M17 helper で delivery の projectID と documentID を取得
	projectID, docID := findAnyDocumentID(t, client, "delivery")
	requirePositiveID(t, projectID, "findAnyDocumentID.projectID")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")

	// Step 2: GetDeliveryRaw で生レスポンスを取得
	getRaw, _, err := client.GetDeliveryRaw(ctx, docID)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("GetDeliveryRaw(%d): %v", docID, err)
	}

	// Step 3: 生 JSON をアーティファクトとして保存（best-effort）
	dumpJSON(t, "deliveries", docID, getRaw)

	// Step 4: 厳格フィールド突合 — DeliveryEntity に未マップフィールドがあれば失敗
	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.DeliveryEntity{}); len(diff) > 0 {
		t.Errorf("deliveries get(%d) unmapped fields: %v", docID, diff)
	}

	// Step 5: エンティティに Unmarshal して基本フィールド検証
	var got boardapi.DeliveryEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != docID {
		t.Errorf("GetDelivery ID mismatch: got=%d want=%d", got.ID, docID)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	var msgLen int
	if got.Message != nil {
		msgLen = len(*got.Message)
	}
	t.Logf("TestE2E_Deliveries_Get: id=%d message_len=%d total=%s tax=%s details_count=%d delivery_date=%s",
		got.ID, msgLen, got.Total, got.Tax, len(got.Details), got.DeliveryDate)
}
