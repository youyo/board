import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";
import type { ClientBranchRef, ClientRef, CompanyBranchRef, ContactRef, UserRef } from "./refs";

/**
 * 請求書。GET /v1/invoices の応答 1 要素。
 * BOARD の OpenAPI(v1.9.0)の応答項目だけを持つ。顧客は入れ子の `client`、ステータスは数値の `invoice_status`
 * (名称は `invoice_status_name`)。`client_id` / `status` は無い(issue #27)。金額は文字列("110000.0")。
 * 必須にしたのは find 層・キャッシュの絞り込みが読む項目と共通項目だけで、残りは response_group によって入らないことがある。
 */
export interface InvoiceEntity {
  id: number;
  project_id: number;
  project_no?: number | null;
  management_no?: string | null;
  /** 案件名 */
  name: string;
  client: ClientRef | null;
  client_branch?: ClientBranchRef | null;
  contact?: ContactRef | null;
  company_branch?: CompanyBranchRef | null;
  user?: UserRef | null;
  total?: string | null;
  tax?: string | null;
  cost_total?: number;
  cost_tax?: number;
  invoice_date?: string | null;
  payment_limit_date?: string | null;
  order_status: number;
  order_status_name: string;
  /** 請求ステータス: 1 未請求 / 4 請求OK / 2 請求済 / 5 一部入金済 / 3 入金済 / 9 回収不能 */
  invoice_status: number;
  invoice_status_name: string;
  payment_method_kbn?: number;
  payment_method_kbn_name?: string;
  project_type_id?: number | null;
  project_type_name?: string | null;
  project_type2_id?: number | null;
  project_type2_name?: string | null;
  project_type3_id?: number | null;
  project_type3_name?: string | null;
  group_id?: number | null;
  group_name?: string | null;
  paid_date?: string | null;
  tags?: string[];
  accounting_type_id?: number | null;
  accounting_type_name?: string | null;
  accounting_type2_id?: number | null;
  accounting_type2_name?: string | null;
  accounting_type3_id?: number | null;
  accounting_type3_name?: string | null;
  lock_flg?: number;
  currency?: string;
  lang_kbn?: number;
  lang_kbn_name?: string;
  exchange_rate?: string | null;
  total_jpy?: number;
  tax_jpy?: number;
  created_at: string;
  updated_at: string;
  /** response_group で書類を付けたときの請求書の本体 */
  invoice?: unknown;
}

/** 請求ステータス(数値コード → 名称)。`invoice_status_in[]` に送る値。 */
export const INVOICE_STATUSES = {
  1: "未請求",
  4: "請求OK",
  2: "請求済",
  5: "一部入金済",
  3: "入金済",
  9: "回収不能",
} as const;

export interface InvoiceListOptions extends CommonListOptions {
  /** 顧客 ID の完全一致(project_client_id_eq) */
  projectClientIdEq?: number;
  /** 請求ステータス(invoice_status_in[]) */
  invoiceStatusIn?: number[];
  /** "small" / "large" */
  responseGroup?: string;
}

export function buildInvoicesQuery(
  opts: InvoiceListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("project_client_id", opts.projectClientIdEq)
    .intIn("invoice_status", opts.invoiceStatusIn)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listInvoices(
  client: BoardClient,
  opts: InvoiceListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<InvoiceEntity>> {
  return listResource<InvoiceEntity, InvoiceListOptions>(
    client,
    "/v1/invoices",
    buildInvoicesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getInvoice(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<InvoiceEntity>> {
  return getResource<InvoiceEntity>(client, "/v1/invoices", id, undefined, "id", ctx);
}
