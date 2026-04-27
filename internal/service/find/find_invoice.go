package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindInvoice は ID / ClientID / Status による請求書横断検索を行う。
// ID 指定時は他を無視し直接 lookup。それ以外は ClientID + Status を
// API 側 (ClientIDEq + StatusEq) で AND 評価する。
//
// Status / Statuses の扱い:
//   - Status (single) は StatusEq で API delegation 可（full-scan 不要）
//   - Statuses (multi) は API 側 StatusIn[] が不在のため post-filter（filterByStatuses）
//   - Statuses-only クエリは validate() で reject 済（N07a D2）
//
// enrichment ポリシー（N04/N05/N06 規約踏襲）:
//   - 主検索（invoices.GetByID / Search）失敗は fail-fast
//   - resolveClientAndProject の失敗は non-fatal（slog.Warn + nil でフィールド埋め）
//
// ID 検索時は Status post-filter を skip（N05 踏襲、UX 配慮）。
func (s *Service) FindInvoice(ctx context.Context, q FindInvoiceQuery) ([]InvoiceResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var invoices []boardapi.InvoiceEntity
	if q.ID != 0 {
		i, err := s.invoices.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		invoices = []boardapi.InvoiceEntity{*i}
	} else {
		list, err := s.invoices.Search(ctx, boardapi.InvoiceListOptions{
			ClientIDEq: q.ClientID,
			StatusEq:   q.Status,
		}, opts)
		if err != nil {
			return nil, err
		}
		invoices = list
	}

	// post-filter: Statuses (multi) のみ。Status (single) は API delegation 済。
	// ID 検索時は skip（N05 踏襲、UX 配慮）。
	if q.ID == 0 && len(q.Statuses) > 0 {
		invoices = filterByStatuses(invoices, func(i boardapi.InvoiceEntity) string { return i.Status }, q.Statuses)
	}

	results := make([]InvoiceResult, 0, len(invoices))
	for _, x := range invoices {
		client, project := s.resolveClientAndProject(ctx, x.ClientID, x.ProjectID, opts)
		results = append(results, InvoiceResult{Invoice: x, Client: client, Project: project})
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	for i := range results {
		results[i].URL = documentURL(s.uiBaseURL, results[i].Invoice.ProjectID)
	}
	return results, nil
}
