package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindUser は ID / Name によるユーザー検索を行う。
// 検索フィールド優先順位: ID > Name（ID 指定時は Name を無視、直接 lookup）。
//
// 他リソース（Project/Client/Vendor）への enrichment は不要（UserResult は User 単独）。
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
