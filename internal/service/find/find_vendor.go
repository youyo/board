package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindVendor performs a cross-resource search for vendors, returning
// vendors with their associated branches and contacts.
// Field priority: ID > Name > Text.
func (s *Service) FindVendor(ctx context.Context, q FindVendorQuery) ([]VendorResult, error) {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return nil, errors.New("at least one of ID, Name, or Text must be set")
	}

	opts := repoOpts(q.Opts)

	var vendors []boardapi.VendorEntity

	switch {
	case q.ID != 0:
		v, err := s.vendors.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		vendors = []boardapi.VendorEntity{*v}

	case q.Name != "":
		result, err := s.vendors.Search(ctx, boardapi.VendorListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		vendors = result

	case q.Text != "":
		all, err := s.vendors.ListEntities(ctx, opts, boardapi.VendorListOptions{})
		if err != nil {
			return nil, err
		}
		for _, v := range all {
			if containsText(q.Text, v.Name, v.Code, v.Memo) {
				vendors = append(vendors, v)
			}
		}
	}

	// Resolve branches and contacts for each vendor
	results := make([]VendorResult, 0, len(vendors))
	for _, v := range vendors {
		r, err := s.resolveVendorDetails(ctx, v, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, r)

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// resolveVendorDetails fetches branches and contacts for a single vendor.
func (s *Service) resolveVendorDetails(ctx context.Context, vendor boardapi.VendorEntity, opts repository.ReadOptions) (VendorResult, error) {
	branches, err := s.vendorBranches.Search(ctx, boardapi.VendorBranchListOptions{PayeeIDEq: vendor.ID}, opts)
	if err != nil {
		return VendorResult{}, err
	}

	contacts, err := s.vendorContacts.Search(ctx, boardapi.VendorContactListOptions{PayeeIDEq: vendor.ID}, opts)
	if err != nil {
		return VendorResult{}, err
	}

	return VendorResult{
		Vendor:   vendor,
		Branches: branches,
		Contacts: contacts,
	}, nil
}
