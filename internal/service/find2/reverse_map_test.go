package find2

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// T10: 初回 Lookup で build が実行され、テーブルが構築される
func TestReverseMapper_FirstCall_BuildsTable(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
	}
	stub := &stubProjectRepo{searchResult: projects}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	pid, ok, err := m.Lookup(context.Background(), 999, repository.ReadOptions{})
	assertNoError(t, err)
	if !ok {
		t.Fatal("want ok=true, got false")
	}
	if pid != 500 {
		t.Fatalf("want projectID=500, got %d", pid)
	}
}

// T11: 2 回目の Lookup では build が再実行されない（stub.Search が 1 回のみ呼ばれる）
func TestReverseMapper_CacheHit_SkipsBuild(t *testing.T) {
	var callCount int32
	stub := &stubProjectRepo{
		searchFunc: func(_ context.Context, _ boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			atomic.AddInt32(&callCount, 1)
			return []boardapi.ProjectEntity{
				{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
			}, nil
		},
	}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	_, _, _ = m.Lookup(context.Background(), 999, repository.ReadOptions{})
	_, _, _ = m.Lookup(context.Background(), 999, repository.ReadOptions{})

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("want 1 Search call, got %d", got)
	}
}

// T12: ヒットした docID は対応する projectID を返す
func TestReverseMapper_Lookup_HitsProjectID(t *testing.T) {
	projects := []boardapi.ProjectEntity{
		{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
	}
	stub := &stubProjectRepo{searchResult: projects}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	pid, ok, err := m.Lookup(context.Background(), 999, repository.ReadOptions{})
	assertNoError(t, err)
	if !ok || pid != 500 {
		t.Fatalf("want (500, true, nil), got (%d, %v, %v)", pid, ok, err)
	}
}

// T22: projects.Search がエラーを返す場合は error が caller に伝播する
func TestReverseMapper_SearchError_BubblesUp(t *testing.T) {
	stub := &stubProjectRepo{
		searchFunc: func(_ context.Context, _ boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			return nil, errors.New("search error")
		},
	}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)
	_, _, err := m.Lookup(context.Background(), 999, repository.ReadOptions{})
	assertError(t, err)
}

// T23: 10 goroutine 同時 Lookup → singleflight により Search は 1 回のみ（-race pass 必須）
func TestReverseMapper_ConcurrentCalls_SingleBuild_NoRace(t *testing.T) {
	var callCount int32
	stub := &stubProjectRepo{
		searchFunc: func(_ context.Context, _ boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			atomic.AddInt32(&callCount, 1)
			time.Sleep(50 * time.Millisecond) // I/O 遅延を模倣
			return []boardapi.ProjectEntity{
				{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
			}, nil
		},
	}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = m.Lookup(context.Background(), 999, repository.ReadOptions{})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("singleflight failed: want 1 build call, got %d", got)
	}
}

// T24: docID が未登録の場合は (0, false, nil) を返す
func TestReverseMapper_LookupMiss_ReturnsZero(t *testing.T) {
	stub := &stubProjectRepo{searchResult: []boardapi.ProjectEntity{
		{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
	}}
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	pid, ok, err := m.Lookup(context.Background(), 99999, repository.ReadOptions{})
	assertNoError(t, err)
	if ok {
		t.Fatal("want ok=false for miss, got true")
	}
	if pid != 0 {
		t.Fatalf("want projectID=0 for miss, got %d", pid)
	}
}

// timeout fallback test: context.WithTimeout 1ms + stub sleep 50ms で build タイムアウト
// → ProjectID=0 フォールバック確認（PoC §2 の cold>10s 要件に対応）
func TestReverseMapper_TimeoutFallback_ReturnsZeroProjectID(t *testing.T) {
	stub := &stubProjectRepo{
		searchFunc: func(ctx context.Context, _ boardapi.ProjectListOptions, _ repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
			// 50ms 待機して ctx を確認（timeout は ensureBuilt 内で 10s になるが、
			// ここでは stub 側の ctx cancel を模倣するため ctx チェックを行う）
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return []boardapi.ProjectEntity{
					{ID: 500, Estimate: &boardapi.DocumentSummary{ID: 999}},
				}, nil
			}
		},
	}
	// ensureBuilt は 10s timeout を使うが、ここでは stub の sleep < timeout のため
	// 直接 ctx cancel を使って build 失敗をシミュレートする
	m := newReverseMapper(stub, "estimate", extractEstimateIDs)

	// parent ctx を 1ms でキャンセル
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ctx を確実に期限切れにする

	pid, ok, err := m.Lookup(ctx, 999, repository.ReadOptions{})
	// timeout/cancel は フォールバックで error なし
	assertNoError(t, err)
	// build 失敗 → テーブル未構築 → miss → (0, false, nil)
	if ok {
		t.Fatalf("want ok=false on timeout fallback, got true (pid=%d)", pid)
	}
	if pid != 0 {
		t.Fatalf("want projectID=0 on timeout fallback, got %d", pid)
	}
}

// extractOrderIDs のテスト（Order フィールド）
func TestExtractOrderIDs(t *testing.T) {
	p := boardapi.ProjectEntity{Order: &boardapi.DocumentSummary{ID: 42}}
	ids := extractOrderIDs(p)
	if len(ids) != 1 || ids[0] != 42 {
		t.Fatalf("want [42], got %v", ids)
	}
}

// extractDeliveryIDs のテスト（Deliveries 配列）
func TestExtractDeliveryIDs(t *testing.T) {
	p := boardapi.ProjectEntity{
		Deliveries: []boardapi.DocumentSummary{{ID: 10}, {ID: 20}},
	}
	ids := extractDeliveryIDs(p)
	if len(ids) != 2 {
		t.Fatalf("want 2 ids, got %v", ids)
	}
}

// extractReceiptIDs のテスト（Receipts 配列）
func TestExtractReceiptIDs(t *testing.T) {
	p := boardapi.ProjectEntity{
		Receipts: []boardapi.DocumentSummary{{ID: 100}, {ID: 200}},
	}
	ids := extractReceiptIDs(p)
	if len(ids) != 2 {
		t.Fatalf("want 2 ids, got %v", ids)
	}
}
