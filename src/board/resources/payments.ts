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
 * 支払(BOARD API では expenditure_payments)。GET /v1/expenditure_payments の応答 1 要素(Go の PaymentEntity)。
 * 注意: Go 版の構造体をそのまま写した。実 API の応答は未知のフィールドも含むので、生データを信頼すること。
 */
export interface PaymentEntity {
  id: number;
  vendor_id: number;
  purchase_order_id: number;
  amount: number;
  status: string;
  payment_date: string;
  memo: string;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PaymentListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(vendor_id_eq) */
  vendorIdEq?: number;
  /** 発注書 ID の完全一致(purchase_order_id_eq) */
  purchaseOrderIdEq?: number;
  /** ステータスの完全一致(status_eq) */
  statusEq?: string;
  /** "small" / "large" */
  responseGroup?: string;
}

export function buildPaymentsQuery(
  opts: PaymentListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("vendor_id", opts.vendorIdEq)
    .intEq("purchase_order_id", opts.purchaseOrderIdEq)
    .strEq("status", opts.statusEq)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listPayments(
  client: BoardClient,
  opts: PaymentListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<PaymentEntity>> {
  return listResource<PaymentEntity, PaymentListOptions>(
    client,
    "/v1/expenditure_payments",
    buildPaymentsQuery,
    opts,
    paging,
  );
}

export function getPayment(client: BoardClient, id: number): Promise<ItemResult<PaymentEntity>> {
  return getResource<PaymentEntity>(client, "/v1/expenditure_payments", id);
}
