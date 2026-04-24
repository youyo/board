package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// M58: CLI flag completion — 固定列挙値の補完候補テーブル + ヘルパ。
//
// 値は BOARD API 仕様および tmp/e2e-artifacts/ の実 API dump (2420件) から確定。
// `--status-eq` (invoices/payments/purchase_orders) は BOARD API 実仕様書での
// 値列挙が確認できなかったため、本 M58 では補完対象から除外する（推測値を
// 埋めて誤情報を定着させるリスクを避けるため）。

var (
	// `--response-group` 共通（small / large 2値のみ）
	responseGroupCommon = []string{"small", "large"}

	// `api projects list --response-group` 拡張（small/large + 5ドキュメント種別 + all）
	responseGroupProjectsList = []string{"small", "large", "estimate", "order", "delivery", "invoice", "receipt", "all"}

	// `api projects get --response-group` 拡張（5ドキュメント種別 + all、small/large は get に不適）
	responseGroupProjectsGet = []string{"estimate", "order", "delivery", "invoice", "receipt", "all"}

	// `--order-status-in` (projects)。dump: projects_0.json 2420件走査で確定。
	orderStatusMap = map[int]string{
		1: "見積中(高)",
		2: "見積中(中)",
		3: "見積中(低)",
		4: "受注確定",
		5: "受注済",
		8: "見積中(除)",
	}

	// `--delivery-status-in` (projects)。dump 実データから確定。
	deliveryStatusMap = map[int]string{
		1: "未着手",
		2: "着手中",
		3: "納品済",
		4: "検収済",
	}

	// `--invoice-timing-kbn-in` (projects)。個別 project dump から確定。
	invoiceTimingKbnMap = map[int]string{
		1: "一括請求",
		2: "定期請求",
	}
)

// staticCompletion は string スライスから cobra の CompletionFunc を作る。
// 引数の slice は防御的にコピーされ、外部からの書き換えで補完候補が変わらない。
func staticCompletion(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	out := make([]string, len(values))
	copy(out, values)
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// intMapCompletion は int→description マップから "<value>\t<description>" 形式で
// 補完候補を返す CompletionFunc を作る。キーは昇順にソートされる。
// cobra は zsh 補完時に "value\tdescription" を解釈し、候補と説明を両方表示する。
// （bash は description を無視し value のみ表示する）
func intMapCompletion(m map[int]string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%d\t%s", k, m[k]))
	}
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
