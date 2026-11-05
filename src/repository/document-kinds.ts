/** 書類 4 種(見積・受注・納品・領収)の対応表。逆引きと find 層が共有する。 */
import type { ProjectEntity } from "../board/resources";

export const DOCUMENT_KINDS = ["estimate", "order", "delivery", "receipt"] as const;
export type DocumentKind = (typeof DOCUMENT_KINDS)[number];

export interface DocumentKindInfo {
  kind: DocumentKind;
  /** repository のキー。 */
  resource: "estimates" | "orders" | "deliveries" | "receipts";
  /** projects の `response_group` に渡す値(種別名と同じ)。 */
  group: DocumentKind;
  /** response_group 付きの案件に埋め込まれた、その種別の書類 ID(0 は除く)。 */
  ids: (p: ProjectEntity) => number[];
}

/**
 * 上流(BOARD)の値は信用しない。ID は 1 以上の安全な整数だけを使う(0・負数・小数・NaN・巨大な値・数値以外は除く)。
 * 逆引きの表・URL に入る前の共通の門。
 */
export const isValidId = (x: unknown): x is number => Number.isSafeInteger(x) && (x as number) >= 1;

const positive = (ids: unknown[]) => ids.filter(isValidId);

export const DOCUMENT_KIND_INFO: Record<DocumentKind, DocumentKindInfo> = {
  estimate: {
    kind: "estimate",
    resource: "estimates",
    group: "estimate",
    ids: (p) => positive([p.estimate?.id]),
  },
  order: {
    kind: "order",
    resource: "orders",
    group: "order",
    ids: (p) => positive([p.order?.id]),
  },
  delivery: {
    kind: "delivery",
    resource: "deliveries",
    group: "delivery",
    ids: (p) => positive((p.deliveries ?? []).map((d) => d.id)),
  },
  receipt: {
    kind: "receipt",
    resource: "receipts",
    group: "receipt",
    ids: (p) => positive((p.receipts ?? []).map((d) => d.id)),
  },
};
