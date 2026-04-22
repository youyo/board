package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindClient performs a cross-resource search for clients, returning
// clients with their associated branches and contacts.
// Field priority: ID > Name > Text.
func (s *Service) FindClient(ctx context.Context, q FindClientQuery) ([]ClientResult, error) {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return nil, errors.New("at least one of ID, Name, or Text must be set")
	}

	opts := repoOpts(q.Opts)

	var clients []boardapi.ClientEntity

	switch {
	case q.ID != 0:
		// ID has highest priority: direct lookup
		c, err := s.clients.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		clients = []boardapi.ClientEntity{*c}

	case q.Name != "":
		// Name search: delegate to repo
		result, err := s.clients.Search(ctx, boardapi.ClientSearchParams{Name: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		clients = result

	case q.Text != "":
		// Text search: list all, filter by name/code/memo
		all, err := s.clients.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range all {
			if containsText(q.Text, c.Name, derefString(c.CustomNo), derefString(c.Note)) {
				clients = append(clients, c)
			}
		}
	}

	// Resolve branches and contacts for each client
	results := make([]ClientResult, 0, len(clients))
	for _, c := range clients {
		r, err := s.resolveClientDetails(ctx, c, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, r)

		// Apply limit
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}

// resolveClientDetails fetches branches and contacts for a single client.
func (s *Service) resolveClientDetails(ctx context.Context, client boardapi.ClientEntity, opts repository.ReadOptions) (ClientResult, error) {
	branches, err := s.clientBranches.Search(ctx, boardapi.ClientBranchSearchParams{ClientID: client.ID}, opts)
	if err != nil {
		return ClientResult{}, err
	}

	contacts, err := s.contacts.Search(ctx, boardapi.ContactSearchParams{ClientID: client.ID}, opts)
	if err != nil {
		return ClientResult{}, err
	}

	return ClientResult{
		Client:   client,
		Branches: branches,
		Contacts: contacts,
	}, nil
}
