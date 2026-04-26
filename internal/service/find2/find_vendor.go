package find2

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindVendor は仕入先を検索し、enrichment 済みの結果を返す。
//
// 検索優先順位: ID > Name > Text（排他、上位が設定されていれば下位は使用しない）。
// Limit > 0 の場合、enrichment 後の件数が Limit に達したらループを打ち切る。
// enrichment（branches / contacts 取得）の失敗は non-fatal（resolveVendorDetails 参照）。
//
// Text マッチ対象: Vendor.Name、Vendor.Code、Vendor.Memo（非ポインタ string）。
func (s *Service) FindVendor(ctx context.Context, q FindVendorQuery) ([]VendorResult, error) {
	// 規約: 全 Find メソッドは validateQuery(q.FindCommonOpts, q) を最初に呼ぶ。
	// q.validate() 単体では FindCommonOpts.validate が走らないため。
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var vendors []boardapi.VendorEntity
	switch {
	case q.ID != 0:
		v, err := s.vendors.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		vendors = []boardapi.VendorEntity{*v}
	case q.Name != "":
		list, err := s.vendors.Search(ctx, boardapi.VendorListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		vendors = list
	case q.Text != "":
		all, err := s.vendors.Search(ctx, boardapi.VendorListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, v := range all {
			// VendorEntity.Code / Memo は非ポインタ string のため derefString 不要
			if containsText(q.Text, v.Name, v.Code, v.Memo) {
				vendors = append(vendors, v)
			}
		}
	}

	results := make([]VendorResult, 0, len(vendors))
	for _, v := range vendors {
		// resolveVendorDetails は non-fatal: err を返さない
		results = append(results, s.resolveVendorDetails(ctx, v, opts))
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}
