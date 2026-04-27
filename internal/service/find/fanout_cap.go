package find

import "fmt"

// fanoutResolveCap は fanout 検索の resolver（client_name / project_name 名前解決）が
// 一度に許容するマッチ件数の上限。超過時はエラーで narrow を要求する。
//
// 背景: BOARD API は 3 req/sec / 3000 req/day の制約があり、resolver が大量マッチした場合
// 後段の Document fetch で rate limit を踏み抜く。MCP/CLI 利用者には narrow を促すほうが
// 結果的に速くデータに到達できる。
const fanoutResolveCap = 5

func errFanoutTooMany(field, value string, n int) error {
	return fmt.Errorf("%s %q matched %d entities (cap=%d); narrow the query or specify a more specific id/project_id to reduce fanout", field, value, n, fanoutResolveCap)
}
