/**
 * contract_status の alias(Go 版 `contract_status.go`)。業務用語の `active` / `ended` / `prospect` / `all` を
 * BOARD のステータス名の集合に展開する。fields-aware: active / ended は納品ステータス名だけ、prospect は受注ステータス名だけで評価する。
 */
import type { ProjectEntity } from "../board/resources";
import { ArgumentError } from "../errors";

export const CONTRACT_STATUSES = ["active", "ended", "prospect", "all"] as const;

const DELIVERY_BY_ALIAS: Record<"active" | "ended", readonly string[]> = {
  active: ["未着手", "着手中", "納品済"],
  ended: ["検収済"],
};
const ORDER_PROSPECT: readonly string[] = ["見積中(高)", "見積中(中)", "見積中(低)", "見積中(除)"];

export interface ExpandedContractStatus {
  delivery: string[];
  order: string[];
}

/** alias は大文字小文字と前後の空白を無視する。空は何もしない。未知の値は argument_error。 */
export function expandContractStatus(alias: string): ExpandedContractStatus {
  switch (alias.trim().toLowerCase()) {
    case "":
      return { delivery: [], order: [] };
    case "active":
      return { delivery: [...DELIVERY_BY_ALIAS.active], order: [] };
    case "ended":
      return { delivery: [...DELIVERY_BY_ALIAS.ended], order: [] };
    case "prospect":
      return { delivery: [], order: [...ORDER_PROSPECT] };
    case "all":
      return {
        delivery: [...DELIVERY_BY_ALIAS.active, ...DELIVERY_BY_ALIAS.ended],
        order: [...ORDER_PROSPECT],
      };
    default:
      throw new ArgumentError(
        `unknown contract_status "${alias}" (valid: active / ended / prospect / all)`,
      );
  }
}

export function filterProjectsByContractStatus(
  projects: ProjectEntity[],
  alias: string,
): ProjectEntity[] {
  const { delivery, order } = expandContractStatus(alias);
  if (delivery.length === 0 && order.length === 0) return projects;
  const d = new Set(delivery);
  const o = new Set(order);
  return projects.filter((p) => d.has(p.delivery_status_name) || o.has(p.order_status_name));
}
