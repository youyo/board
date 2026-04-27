package find

import (
	"fmt"
	"strings"

	"github.com/youyo/board/internal/boardapi"
)

// ContractStatus alias 定数。MCP/CLI 層から参照可能な公開定数。
const (
	ContractStatusActive   = "active"
	ContractStatusEnded    = "ended"
	ContractStatusProspect = "prospect"
	ContractStatusAll      = "all"
)

// alias ごとの DeliveryStatusName マッピング。
// アクティブ案件（進行中）は DeliveryStatusName で判定。
var deliveryStatusByAlias = map[string][]string{
	ContractStatusActive: {"未着手", "着手中", "納品済"},
	ContractStatusEnded:  {"検収済"},
}

// alias ごとの OrderStatusName マッピング。
// 見込み案件は OrderStatusName で判定。
var orderStatusByAlias = map[string][]string{
	ContractStatusProspect: {"見積中(高)", "見積中(中)", "見積中(低)", "見積中(除)"},
}

// expandContractStatus は alias 文字列を DeliveryStatusName 集合と OrderStatusName 集合に展開する。
// alias は大文字小文字を無視し、前後空白を除去して正規化する。
// 空 alias は no-op（エラーなし、空スライス返却）。
// 不正値は "unknown contract_status %q (valid: active / ended / prospect / all)" エラー。
func expandContractStatus(alias string) (matchDelivery []string, matchOrder []string, err error) {
	normalized := strings.ToLower(strings.TrimSpace(alias))
	if normalized == "" {
		return nil, nil, nil
	}

	switch normalized {
	case ContractStatusActive:
		return deliveryStatusByAlias[ContractStatusActive], nil, nil
	case ContractStatusEnded:
		return deliveryStatusByAlias[ContractStatusEnded], nil, nil
	case ContractStatusProspect:
		return nil, orderStatusByAlias[ContractStatusProspect], nil
	case ContractStatusAll:
		// active∪ended の DeliveryStatusName + prospect の OrderStatusName
		md := append(
			append([]string{}, deliveryStatusByAlias[ContractStatusActive]...),
			deliveryStatusByAlias[ContractStatusEnded]...,
		)
		mo := append([]string{}, orderStatusByAlias[ContractStatusProspect]...)
		return md, mo, nil
	default:
		return nil, nil, fmt.Errorf(
			"unknown contract_status %q (valid: active / ended / prospect / all)",
			alias,
		)
	}
}

// filterProjectsByContractStatus は alias に応じて fields-aware なフィルタを適用する。
// active / ended は DeliveryStatusName のみで評価。
// prospect は OrderStatusName のみで評価。
// all は両フィールドで評価（active∪ended∪prospect）。
func filterProjectsByContractStatus(projects []boardapi.ProjectEntity, alias string) ([]boardapi.ProjectEntity, error) {
	matchDelivery, matchOrder, err := expandContractStatus(alias)
	if err != nil {
		return nil, err
	}
	if len(matchDelivery) == 0 && len(matchOrder) == 0 {
		return projects, nil
	}

	deliverySet := setFromSlice(matchDelivery)
	orderSet := setFromSlice(matchOrder)

	out := make([]boardapi.ProjectEntity, 0, len(projects))
	for _, p := range projects {
		if _, ok := deliverySet[p.DeliveryStatusName]; ok {
			out = append(out, p)
			continue
		}
		if _, ok := orderSet[p.OrderStatusName]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// setFromSlice は文字列スライスを集合（map）に変換するヘルパー。
func setFromSlice(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}
