package find2

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindUser は ID / Name / Text によるユーザー検索を行う。
// 検索フィールド優先順位: ID > Name > Text（排他）。
//
// 他リソース（Project/Client/Vendor）への enrichment は不要（UserResult は User 単独）。
//
// Text マッチ対象: Name, LastName, FirstName, Email（全て非ポインタ string）。
//
// Limit > 0 の場合、返却件数が Limit に達したらループを打ち切る。
func (s *Service) FindUser(ctx context.Context, q FindUserQuery) ([]UserResult, error) {
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var users []boardapi.UserEntity
	switch {
	case q.ID != 0:
		u, err := s.users.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		users = []boardapi.UserEntity{*u}
	case q.Name != "":
		list, err := s.users.Search(ctx, boardapi.UserListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		users = list
	case q.Text != "":
		all, err := s.users.Search(ctx, boardapi.UserListOptions{}, opts)
		if err != nil {
			return nil, err
		}
		for _, x := range all {
			if containsText(q.Text, x.Name, x.LastName, x.FirstName, x.Email) {
				users = append(users, x)
			}
		}
	}

	results := make([]UserResult, 0, len(users))
	for _, x := range users {
		results = append(results, UserResult{User: x})
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}
