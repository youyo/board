package find2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
	"golang.org/x/sync/singleflight"
)

// reverseMapper は Document 種別ごとに documentID → projectID のマッピングテーブルを
// lazy build + singleflight で管理する（N02 §5.3）。
// cold build は全プロジェクトの Search を行うため時間がかかる（実測 >10s）。
// ctx timeout フォールバック（ProjectID=0）を必ず実装する。
type reverseMapper struct {
	projects      ProjectRepo
	responseGroup string
	extractDocIDs func(p boardapi.ProjectEntity) []int

	mu    sync.RWMutex
	table map[int]int // documentID → projectID
	built bool

	sf singleflight.Group
}

// newReverseMapper は reverseMapper を生成する。build は Lookup 時に lazy に行われる。
func newReverseMapper(
	p ProjectRepo,
	responseGroup string,
	extractDocIDs func(p boardapi.ProjectEntity) []int,
) *reverseMapper {
	return &reverseMapper{
		projects:      p,
		responseGroup: responseGroup,
		extractDocIDs: extractDocIDs,
	}
}

// Lookup は docID に対応する projectID を返す。
// 未構築の場合は singleflight で build を 1 本に集約してから参照する。
// build が 10s を超えてタイムアウトした場合は (0, false, nil) を返す（ProjectID=0 フォールバック）。
func (m *reverseMapper) Lookup(
	ctx context.Context,
	docID int,
	opts repository.ReadOptions,
) (projectID int, ok bool, err error) {
	if err := m.ensureBuilt(ctx, opts); err != nil {
		return 0, false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	pid, ok := m.table[docID]
	return pid, ok, nil
}

// ensureBuilt はテーブルが未構築の場合に singleflight で build を実行する。
// build に 10s 以上かかる場合は ctx timeout を発火させ (0, false, nil) を返す。
func (m *reverseMapper) ensureBuilt(ctx context.Context, opts repository.ReadOptions) error {
	m.mu.RLock()
	built := m.built
	m.mu.RUnlock()
	if built {
		return nil
	}

	// singleflight で build を 1 本に集約する。
	// build 用に 10s timeout 付きの ctx を用意する（PoC 実測: 全 4 種で >10s）。
	_, sfErr, _ := m.sf.Do("build", func() (any, error) {
		buildCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		slog.Info("[SLOW:cold-reverse-map] building reverse map",
			"response_group", m.responseGroup)

		err := m.buildUnlocked(buildCtx, opts)
		if err != nil {
			// ctx timeout / cancel はフォールバック（エラーを上位に返さない）
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				slog.Warn("find2.reverseMapper: build timed out, falling back to ProjectID=0",
					"response_group", m.responseGroup)
				return nil, nil // フォールバック: エラーとして扱わない
			}
			return nil, err
		}
		return nil, nil
	})
	return sfErr
}

// buildUnlocked は projects.Search を呼び出してテーブルを構築する。
// エラーは fmt.Errorf でラップして返す。
func (m *reverseMapper) buildUnlocked(ctx context.Context, opts repository.ReadOptions) error {
	list, err := m.projects.Search(
		ctx,
		boardapi.ProjectListOptions{ResponseGroup: m.responseGroup},
		opts,
	)
	if err != nil {
		return fmt.Errorf("reverseMapper build (%s): %w", m.responseGroup, err)
	}
	t := make(map[int]int, len(list))
	for _, p := range list {
		for _, id := range m.extractDocIDs(p) {
			if id != 0 {
				t[id] = p.ID
			}
		}
	}
	m.mu.Lock()
	m.table = t
	m.built = true
	m.mu.Unlock()
	return nil
}

// ========== extractDocIDs: Document 4 種の ID 抽出 closure ==========

// extractEstimateIDs は ProjectEntity の Estimate から見積 ID を抽出する。
func extractEstimateIDs(p boardapi.ProjectEntity) []int {
	if p.Estimate == nil {
		return nil
	}
	return []int{p.Estimate.ID}
}

// extractOrderIDs は ProjectEntity の Order から注文 ID を抽出する。
func extractOrderIDs(p boardapi.ProjectEntity) []int {
	if p.Order == nil {
		return nil
	}
	return []int{p.Order.ID}
}

// extractDeliveryIDs は ProjectEntity の Deliveries から納品 ID 群を抽出する。
func extractDeliveryIDs(p boardapi.ProjectEntity) []int {
	ids := make([]int, 0, len(p.Deliveries))
	for _, d := range p.Deliveries {
		ids = append(ids, d.ID)
	}
	return ids
}

// extractReceiptIDs は ProjectEntity の Receipts から領収 ID 群を抽出する。
func extractReceiptIDs(p boardapi.ProjectEntity) []int {
	ids := make([]int, 0, len(p.Receipts))
	for _, r := range p.Receipts {
		ids = append(ids, r.ID)
	}
	return ids
}
