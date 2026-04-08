package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindGroup performs a search for groups.
// Field priority: ID > Name > Text.
func (s *Service) FindGroup(ctx context.Context, q FindGroupQuery) ([]GroupResult, error) {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return nil, errors.New("at least one of ID, Name, or Text must be set")
	}

	opts := repoOpts(q.Opts)

	var groups []boardapi.GroupEntity

	switch {
	case q.ID != 0:
		g, err := s.groups.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		groups = []boardapi.GroupEntity{*g}

	case q.Name != "":
		result, err := s.groups.Search(ctx, boardapi.GroupSearchParams{Name: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		groups = result

	case q.Text != "":
		all, err := s.groups.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, g := range all {
			if containsText(q.Text, g.Name, g.Memo) {
				groups = append(groups, g)
			}
		}
	}

	results := make([]GroupResult, 0, len(groups))
	for _, g := range groups {
		results = append(results, GroupResult{Group: g})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}
