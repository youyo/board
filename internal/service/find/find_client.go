package find

import (
	"context"

	"github.com/youyo/board/internal/boardapi"
)

// FindClient はクライアントを検索し、enrichment 済みの結果を返す。
//
// 検索優先順位: ID > Name（ID 指定時は Name を無視、直接 lookup）。
// Limit > 0 の場合、enrichment 後の件数が Limit に達したらループを打ち切る。
// enrichment（branches / contacts 取得）の失敗は non-fatal（resolveClientDetails 参照）。
func (s *Service) FindClient(ctx context.Context, q FindClientQuery) ([]ClientResult, error) {
	// 規約: 全 Find メソッドは validateQuery(q.FindCommonOpts, q) を最初に呼ぶ。
	// q.validate() 単体では FindCommonOpts.validate が走らないため。
	if err := validateQuery(q.FindCommonOpts, q); err != nil {
		return nil, err
	}
	opts := repoOpts(q.FindCommonOpts)

	var clients []boardapi.ClientEntity
	switch {
	case q.ID != 0:
		c, err := s.clients.GetByID(ctx, q.ID, opts)
		if err != nil {
			return nil, err
		}
		clients = []boardapi.ClientEntity{*c}
	case q.Name != "":
		list, err := s.clients.Search(ctx, boardapi.ClientListOptions{NameCont: q.Name}, opts)
		if err != nil {
			return nil, err
		}
		clients = list
	}

	results := make([]ClientResult, 0, len(clients))
	for _, c := range clients {
		// resolveClientDetails は non-fatal: err を返さない
		results = append(results, s.resolveClientDetails(ctx, c, opts))
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	for i := range results {
		results[i].URL = clientURL(s.uiBaseURL, results[i].Client.ID)
	}
	return results, nil
}
