package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindPurchaseOrder performs a cross-resource search for purchase orders, returning
// purchase orders with their associated vendor and project.
// Field priority: ID > VendorName > ProjectName > Text > Status(standalone).
// Status also acts as a post-filter when combined with other criteria.
func (s *Service) FindPurchaseOrder(ctx context.Context, q FindPurchaseOrderQuery) ([]PurchaseOrderResult, error) {
	if q.ID == 0 && q.VendorName == "" && q.ProjectName == "" && q.Text == "" && q.Status == "" {
		return nil, errors.New("at least one of ID, VendorName, ProjectName, Text, or Status must be set")
	}

	opts := repoOpts(q.Opts)

	var purchaseOrders []boardapi.PurchaseOrderEntity

	switch {
	case q.ID != 0:
		po, err := s.purchaseOrders.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		purchaseOrders = []boardapi.PurchaseOrderEntity{*po}

	case q.VendorName != "":
		vendors, err := s.vendors.Search(ctx, boardapi.VendorSearchParams{Name: q.VendorName}, opts)
		if err != nil {
			return nil, err
		}
		for _, v := range vendors {
			pos, err := s.purchaseOrders.Search(ctx, boardapi.PurchaseOrderListOptions{VendorIDEq: v.ID}, opts)
			if err != nil {
				return nil, err
			}
			purchaseOrders = append(purchaseOrders, pos...)
		}

	case q.ProjectName != "":
		projects, err := s.projects.Search(ctx, boardapi.ProjectListOptions{NameCont: q.ProjectName}, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			pos, err := s.purchaseOrders.Search(ctx, boardapi.PurchaseOrderListOptions{ProjectIDEq: p.ID}, opts)
			if err != nil {
				return nil, err
			}
			purchaseOrders = append(purchaseOrders, pos...)
		}

	case q.Text != "":
		all, err := s.purchaseOrders.Search(ctx, boardapi.PurchaseOrderListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, po := range all {
			if containsText(q.Text, po.Title, po.Memo) {
				purchaseOrders = append(purchaseOrders, po)
			}
		}

	case q.Status != "":
		all, err := s.purchaseOrders.Search(ctx, boardapi.PurchaseOrderListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, po := range all {
			if po.Status == q.Status {
				purchaseOrders = append(purchaseOrders, po)
			}
		}
	}

	// Apply status post-filter for non-status-only search modes
	if q.Status != "" && q.ID == 0 && (q.VendorName != "" || q.ProjectName != "" || q.Text != "") {
		purchaseOrders = filterPurchaseOrdersByStatus(purchaseOrders, q.Status)
	}

	// Build results with vendor/project resolution
	results := make([]PurchaseOrderResult, 0, len(purchaseOrders))
	for _, po := range purchaseOrders {
		vendor, project := s.resolveVendorAndProject(ctx, po.VendorID, po.ProjectID, opts)
		results = append(results, PurchaseOrderResult{
			PurchaseOrder: po,
			Vendor:        vendor,
			Project:       project,
		})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// filterPurchaseOrdersByStatus filters purchase orders by status.
func filterPurchaseOrdersByStatus(pos []boardapi.PurchaseOrderEntity, status string) []boardapi.PurchaseOrderEntity {
	filtered := make([]boardapi.PurchaseOrderEntity, 0, len(pos))
	for _, po := range pos {
		if po.Status == status {
			filtered = append(filtered, po)
		}
	}
	return filtered
}
