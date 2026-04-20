//go:build e2e

// E2E tests for /v1/documents/estimates against the real BOARD API.
//
// Scope (M18/M35, board-compliance roadmap): Phase G (document) 2nd milestone.
// estimates は document 系エンドポイントで Get のみ提供（List/Search は対象外）。
// documentID discovery は M17 で確立した findAnyDocumentID helper を経由する。
//
//   - Get (TestE2E_Estimates_Get) : ~3 req expected
//     (ListProjectsPage 1 + GetProjectWithGroupRaw 1 + GetEstimateRaw 1)
//
// Total budget: ~5 req (plan cap 8).
// 1 endpoint 1 execution. No skip on 403/429: the BOARD compliance roadmap
// requires immediate failure so that environment or permission issues are not
// silently masked.
//
// Data-dependent skip: when findAnyDocumentID cannot find any estimate in the
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
//	    -run TestE2E_Estimates_Get ./internal/boardapi/

package boardapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/testhelper"
)

// TestE2E_Estimates_Get discovers an estimate document ID via M17 helper,
// fetches the detail via GetEstimateRaw, and applies strict field diff on the
// single-object response.
//
// Discovery is delegated to findAnyDocumentID(t, client, "estimate"), which
// scans the top maxDiscoveryProjects projects with response_group=estimate.
// Data-dependent skip is handled inside the helper.
// A 403 or 429 on the discovered ID surfaces via t.Fatalf (not skipped),
// per the BOARD compliance roadmap rule: no skip on 403/429.
func TestE2E_Estimates_Get(t *testing.T) {
	client := newE2EClient(t)
	ctx := context.Background()

	// Step 1: M17 helper で estimate の projectID と documentID を取得
	projectID, docID := findAnyDocumentID(t, client, "estimate")
	requirePositiveID(t, projectID, "findAnyDocumentID.projectID")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")

	// Step 2: GetEstimateRaw で生レスポンスを取得
	getRaw, err := client.GetEstimateRaw(ctx, docID)
	if err != nil {
		// Roadmap rule: 403/429 must NOT be skipped, they must fail the test.
		t.Fatalf("GetEstimateRaw(%d): %v", docID, err)
	}

	// Step 3: 生 JSON をアーティファクトとして保存（best-effort）
	dumpJSON(t, "estimates", docID, getRaw)

	// Step 4: 厳格フィールド突合 — EstimateEntity に未マップフィールドがあれば失敗
	if diff := testhelper.StrictFieldDiff(t, getRaw, &boardapi.EstimateEntity{}); len(diff) > 0 {
		t.Errorf("estimates get(%d) unmapped fields: %v", docID, diff)
	}

	// Step 5: エンティティに Unmarshal して基本フィールド検証
	var got boardapi.EstimateEntity
	if err := json.Unmarshal(getRaw, &got); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if got.ID != docID {
		t.Errorf("GetEstimate ID mismatch: got=%d want=%d", got.ID, docID)
	}
	// Log only lengths and numeric IDs (non-PII) for traceability.
	// Do NOT log message text raw values.
	var msgLen int
	if got.Message != nil {
		msgLen = len(*got.Message)
	}
	t.Logf("TestE2E_Estimates_Get: id=%d message_len=%d total=%s tax=%s details_count=%d valid_period_len=%d",
		got.ID, msgLen, got.Total, got.Tax, len(got.Details), len(got.ValidPeriod))
}
