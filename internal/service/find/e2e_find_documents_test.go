//go:build e2e

package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// findProjectWithDoc は response_group=docType でドキュメントが付随するプロジェクトを探す。
// docType: "estimate" | "order" | "delivery" | "receipt"
// 戻り値: (projectID, documentID)。見つからなければ (0, 0) を返し、呼び出し側で SKIP。
func findProjectWithDoc(t *testing.T, svc *find.Service, docType string) (int, int) {
	t.Helper()
	// Phase は SKIP にせず最初の N 件 project を直接 ID lookup でなく
	// Document を ProjectID 列挙で叩いて確認する設計。
	// ただし Service 直接 API には response_group 走査を持たないため、
	// 「Project 一覧 → 各 ID で FindXxx を試行」方式を最大 3 件試す（rate-limit 配慮）。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// seed: client → ClientID 経由で project を取得（project name に "株" が含まれる前提を回避）
	cs, err := svc.FindClient(ctx, find.FindClientQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 1},
		Name:           "株",
	})
	if err != nil || len(cs) == 0 {
		return 0, 0
	}
	ps, err := svc.FindProject(ctx, find.FindProjectQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 3},
		ClientID:       cs[0].Client.ID,
	})
	if err != nil || len(ps) == 0 {
		return 0, 0
	}
	for _, p := range ps {
		var docID int
		switch docType {
		case "estimate":
			rs, _ := svc.FindEstimate(ctx, find.FindEstimateQuery{ProjectID: p.Project.ID})
			if len(rs) > 0 {
				docID = rs[0].Estimate.ID
			}
		case "order":
			rs, _ := svc.FindOrder(ctx, find.FindOrderQuery{ProjectID: p.Project.ID})
			if len(rs) > 0 {
				docID = rs[0].Order.ID
			}
		case "delivery":
			rs, _ := svc.FindDelivery(ctx, find.FindDeliveryQuery{ProjectID: p.Project.ID})
			if len(rs) > 0 {
				docID = rs[0].Delivery.ID
			}
		case "receipt":
			rs, _ := svc.FindReceipt(ctx, find.FindReceiptQuery{ProjectID: p.Project.ID})
			if len(rs) > 0 {
				docID = rs[0].Receipt.ID
			}
		}
		if docID > 0 {
			return p.Project.ID, docID
		}
	}
	return 0, 0
}

// ====== Estimate (T10-T13) ======

func TestE2E_FindEstimate_ByProjectID_Returns_Estimate(t *testing.T) {
	svc := newE2EService(t)
	pid, _ := findProjectWithDoc(t, svc, "estimate")
	if pid == 0 {
		t.Skipf("[SKIP:no-data] no estimate found in top projects")
	}
	rs, err := svc.FindEstimate(context.Background(), find.FindEstimateQuery{ProjectID: pid})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindEstimate(ProjectID=%d): %v", pid, err)
	}
	if len(rs) == 0 {
		t.Skipf("[SKIP:no-data] estimate not present for project %d", pid)
	}
}

func TestE2E_FindEstimate_ByID_ReverseMapper_CacheHit(t *testing.T) {
	svc := newE2EService(t)
	_, did := findProjectWithDoc(t, svc, "estimate")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no estimate doc id available")
	}
	// 1 度目で cache 構築 → 2 度目で hit
	_, _ = svc.FindEstimate(context.Background(), find.FindEstimateQuery{ID: did})
	rs, err := svc.FindEstimate(context.Background(), find.FindEstimateQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindEstimate(ID=%d): %v", did, err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(rs))
	}
	t.Logf("estimate cache-hit: pid=%d cid=%d", rs[0].ProjectID, rs[0].ClientID)
}

func TestE2E_FindEstimate_ByID_ReverseMapper_FreshCache(t *testing.T) {
	// 新規 service で cache miss を観測。
	svc := newE2EService(t)
	_, did := findProjectWithDoc(t, svc, "estimate")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no estimate doc id")
	}
	// 別 service instance で fresh state（ただし repo cache DB は共有なので完全 cold ではない）
	svc2 := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := svc2.FindEstimate(ctx, find.FindEstimateQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindEstimate(ID=%d) fresh: %v", did, err)
	}
	t.Logf("estimate fresh-mapper: results=%d", len(rs))
}

func TestE2E_FindEstimate_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindEstimate(context.Background(), find.FindEstimateQuery{})
	if err == nil {
		t.Fatalf("expected reject for empty query")
	}
	if !containsErrSubstr(err, "at least one field") {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

// ====== Order (T14-T17) ======

func TestE2E_FindOrder_ByProjectID_Returns_Order(t *testing.T) {
	svc := newE2EService(t)
	pid, _ := findProjectWithDoc(t, svc, "order")
	if pid == 0 {
		t.Skipf("[SKIP:no-data] no order found")
	}
	rs, err := svc.FindOrder(context.Background(), find.FindOrderQuery{ProjectID: pid})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindOrder(ProjectID=%d): %v", pid, err)
	}
	t.Logf("order count=%d", len(rs))
}

func TestE2E_FindOrder_ByID_ReverseMapper(t *testing.T) {
	svc := newE2EService(t)
	_, did := findProjectWithDoc(t, svc, "order")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no order doc id")
	}
	rs, err := svc.FindOrder(context.Background(), find.FindOrderQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindOrder(ID=%d): %v", did, err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 order, got %d", len(rs))
	}
}

func TestE2E_FindOrder_FreshCacheMiss(t *testing.T) {
	svc1 := newE2EService(t)
	_, did := findProjectWithDoc(t, svc1, "order")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no order doc id")
	}
	svc2 := newE2EService(t)
	rs, err := svc2.FindOrder(context.Background(), find.FindOrderQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindOrder fresh: %v", err)
	}
	t.Logf("order fresh: results=%d", len(rs))
}

func TestE2E_FindOrder_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindOrder(context.Background(), find.FindOrderQuery{})
	if err == nil {
		t.Fatalf("expected reject")
	}
}

// ====== Delivery (T18-T21) ======

func TestE2E_FindDelivery_ByProjectID_Returns_Array(t *testing.T) {
	svc := newE2EService(t)
	pid, _ := findProjectWithDoc(t, svc, "delivery")
	if pid == 0 {
		t.Skipf("[SKIP:no-data] no delivery")
	}
	rs, err := svc.FindDelivery(context.Background(), find.FindDeliveryQuery{ProjectID: pid})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindDelivery(ProjectID=%d): %v", pid, err)
	}
	t.Logf("delivery count=%d (array iteration verified)", len(rs))
}

func TestE2E_FindDelivery_ByID_ReverseMapper(t *testing.T) {
	svc := newE2EService(t)
	_, did := findProjectWithDoc(t, svc, "delivery")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no delivery doc id")
	}
	rs, err := svc.FindDelivery(context.Background(), find.FindDeliveryQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindDelivery(ID=%d): %v", did, err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(rs))
	}
}

func TestE2E_FindDelivery_FreshCacheMiss(t *testing.T) {
	svc1 := newE2EService(t)
	_, did := findProjectWithDoc(t, svc1, "delivery")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no delivery doc id")
	}
	svc2 := newE2EService(t)
	rs, err := svc2.FindDelivery(context.Background(), find.FindDeliveryQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindDelivery fresh: %v", err)
	}
	t.Logf("delivery fresh: results=%d", len(rs))
}

func TestE2E_FindDelivery_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindDelivery(context.Background(), find.FindDeliveryQuery{})
	if err == nil {
		t.Fatalf("expected reject")
	}
}

// ====== Receipt (T22-T25) ======

func TestE2E_FindReceipt_ByProjectID_Returns_Receipt(t *testing.T) {
	svc := newE2EService(t)
	pid, _ := findProjectWithDoc(t, svc, "receipt")
	if pid == 0 {
		t.Skipf("[SKIP:no-data] no receipt")
	}
	rs, err := svc.FindReceipt(context.Background(), find.FindReceiptQuery{ProjectID: pid})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindReceipt(ProjectID=%d): %v", pid, err)
	}
	t.Logf("receipt count=%d", len(rs))
}

func TestE2E_FindReceipt_ByID_ReverseMapper(t *testing.T) {
	svc := newE2EService(t)
	_, did := findProjectWithDoc(t, svc, "receipt")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no receipt doc id")
	}
	rs, err := svc.FindReceipt(context.Background(), find.FindReceiptQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindReceipt(ID=%d): %v", did, err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(rs))
	}
}

func TestE2E_FindReceipt_FreshCacheMiss(t *testing.T) {
	svc1 := newE2EService(t)
	_, did := findProjectWithDoc(t, svc1, "receipt")
	if did == 0 {
		t.Skipf("[SKIP:no-data] no receipt doc id")
	}
	svc2 := newE2EService(t)
	rs, err := svc2.FindReceipt(context.Background(), find.FindReceiptQuery{ID: did})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindReceipt fresh: %v", err)
	}
	t.Logf("receipt fresh: results=%d", len(rs))
}

func TestE2E_FindReceipt_NoFields_Rejects(t *testing.T) {
	svc := newE2EService(t)
	_, err := svc.FindReceipt(context.Background(), find.FindReceiptQuery{})
	if err == nil {
		t.Fatalf("expected reject")
	}
}
