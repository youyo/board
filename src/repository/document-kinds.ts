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

const positive = (ids: Array<number | undefined>) =>
  ids.filter((x): x is number => typeof x === "number" && x !== 0);

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
