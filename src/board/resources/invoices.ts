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
 * 請求書。GET /v1/invoices の応答 1 要素(Go の InvoiceEntity)。
 * 注意: Go 版の構造体をそのまま写した。実 API の応答は未知のフィールドも含むので、生データを信頼すること。
 */
export interface InvoiceEntity {
  id: number;
  client_id: number;
  project_id: number;
  title: string;
  total_amount: number;
  status: string;
  /** ISO 8601 の日付 */
  invoice_date: string;
  due_date: string;
  memo: string;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface InvoiceListOptions extends CommonListOptions {
  /** 顧客 ID の完全一致(client_id_eq) */
  clientIdEq?: number;
  /** 案件 ID の完全一致(project_id_eq) */
  projectIdEq?: number;
  /** ステータスの完全一致(status_eq) */
  statusEq?: string;
  /** "small" / "large" */
  responseGroup?: string;
}

export function buildInvoicesQuery(
  opts: InvoiceListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("client_id", opts.clientIdEq)
    .intEq("project_id", opts.projectIdEq)
    .strEq("status", opts.statusEq)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listInvoices(
  client: BoardClient,
  opts: InvoiceListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<InvoiceEntity>> {
  return listResource<InvoiceEntity, InvoiceListOptions>(
    client,
    "/v1/invoices",
    buildInvoicesQuery,
    opts,
    paging,
  );
}

export function getInvoice(client: BoardClient, id: number): Promise<ItemResult<InvoiceEntity>> {
  return getResource<InvoiceEntity>(client, "/v1/invoices", id);
}
