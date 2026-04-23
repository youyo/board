package find

import (
	"context"
	"errors"

	"github.com/youyo/board/internal/boardapi"
)

// FindUser performs a search for users.
// Field priority: ID > Name > Text.
func (s *Service) FindUser(ctx context.Context, q FindUserQuery) ([]UserResult, error) {
	if q.ID == 0 && q.Name == "" && q.Text == "" {
		return nil, errors.New("at least one of ID, Name, or Text must be set")
	}

	opts := repoOpts(q.Opts)

	var users []boardapi.UserEntity

	switch {
	case q.ID != 0:
		u, err := s.users.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		users = []boardapi.UserEntity{*u}

	case q.Name != "":
		result, err := s.users.Search(ctx, boardapi.UserListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		users = result

	case q.Text != "":
		all, err := s.users.Search(ctx, boardapi.UserListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, u := range all {
			if containsText(q.Text, u.DisplayName(), u.LastName, u.FirstName, u.Email) {
				users = append(users, u)
			}
		}
	}

	results := make([]UserResult, 0, len(users))
	for _, u := range users {
		results = append(results, UserResult{User: u})

		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}

	return results, nil
}
