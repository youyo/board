import type { BoardClient } from "../client";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";

/**
 * 発注書(BOARD API では expenditures)。GET /v1/expenditures の応答 1 要素(Go の PurchaseOrderEntity)。
 * 注意: Go 版の構造体をそのまま写した。実 API の応答は未知のフィールドも含むので、生データを信頼すること。
 */
export interface PurchaseOrderEntity {
  id: number;
  vendor_id: number;
  project_id: number;
  title: string;
  total_amount: number;
  status: string;
  order_date: string;
  delivery_date: string;
  memo: string;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PurchaseOrderListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(vendor_id_eq) */
  vendorIdEq?: number;
  /** 案件 ID の完全一致(project_id_eq) */
  projectIdEq?: number;
  /** ステータスの完全一致(status_eq) */
  statusEq?: string;
  /** "small" / "large" */
  responseGroup?: string;
}

export function buildPurchaseOrdersQuery(
  opts: PurchaseOrderListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("vendor_id", opts.vendorIdEq)
    .intEq("project_id", opts.projectIdEq)
    .strEq("status", opts.statusEq)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listPurchaseOrders(
  client: BoardClient,
  opts: PurchaseOrderListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<PurchaseOrderEntity>> {
  return listResource<PurchaseOrderEntity, PurchaseOrderListOptions>(
    client,
    "/v1/expenditures",
    buildPurchaseOrdersQuery,
    opts,
    paging,
  );
}

export function getPurchaseOrder(
  client: BoardClient,
  id: number,
): Promise<ItemResult<PurchaseOrderEntity>> {
  return getResource<PurchaseOrderEntity>(client, "/v1/expenditures", id);
}
