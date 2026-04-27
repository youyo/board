package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindPurchaseOrder は ID / VendorID / Status による発注書横断検索を行う。
// ID 指定時は他を無視し直接 lookup。それ以外は VendorID + Status を
// API 側 (VendorIDEq + StatusEq) で AND 評価する。
//
// Status / Statuses の扱い:
//   - Status (single) は StatusEq で API delegation 可（full-scan 不要）
//   - Statuses (multi) は API 側 StatusIn[] が不在のため post-filter（filterByStatuses）
//   - Statuses-only クエリは validate() で reject 済（N07a D2）
//
// enrichment ポリシー（N04/N05/N06 規約踏襲）:
//   - 主検索（purchaseOrders.GetByID / Search）失敗は fail-fast
//   - resolveVendorAndProject の失敗は non-fatal（slog.Warn + nil でフィールド埋め）
//
// ID 検索時は Status post-filter を skip（N05 踏襲、UX 配慮）。
func (s *Service) FindPurchaseOrder(ctx context.Context, q FindPurchaseOrderQuery) ([]PurchaseOrderResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var pos []boardapi.PurchaseOrderEntity
	if q.ID != 0 {
		po, err := s.purchaseOrders.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		pos = []boardapi.PurchaseOrderEntity{*po}
	} else {
		list, err := s.purchaseOrders.Search(ctx, boardapi.PurchaseOrderListOptions{
			VendorIDEq: q.VendorID,
			StatusEq:   q.Status,
		}, opts)
		if err != nil {
			return nil, err
		}
		pos = list
	}

	// post-filter: Statuses (multi) のみ。Status (single) は API delegation 済。
	// ID 検索時は skip（N05 踏襲、UX 配慮）。
	if q.ID == 0 && len(q.Statuses) > 0 {
		pos = filterByStatuses(pos, func(p boardapi.PurchaseOrderEntity) string { return p.Status }, q.Statuses)
	}

	results := make([]PurchaseOrderResult, 0, len(pos))
	for _, x := range pos {
		vendor, project := s.resolveVendorAndProject(ctx, x.VendorID, x.ProjectID, opts)
		results = append(results, PurchaseOrderResult{PurchaseOrder: x, Vendor: vendor, Project: project})
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	for i := range results {
		results[i].URL = documentURL(s.uiBaseURL, results[i].PurchaseOrder.ProjectID)
	}
	return results, nil
}
