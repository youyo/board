import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { QueryBuilder } from "../query";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";
import type { CompanyBranchRef, ProjectRef, UserRef, VendorRef } from "./refs";

/**
 * 発注書(BOARD API では expenditures)。GET /v1/expenditures の応答 1 要素。
 * BOARD の OpenAPI(v1.9.0)の応答項目だけを持つ。支払先は入れ子の `payee`、ステータスは数値の
 * `expenditure_status`(発注)と `delivery_status`(進捗)。`vendor_id` / `status` は無い(issue #27)。
 * `project` / `project_id` は response_group が medium 以上のときだけ入る(一覧の既定は medium)。
 */
export interface PurchaseOrderEntity {
  id: number;
  /** 発注 No */
  expenditure_no: number;
  name: string;
  payee_id?: number | null;
  payee: VendorRef | null;
  payee_branch_id?: number | null;
  payee_branch?: Record<string, unknown> | null;
  payee_name_disp_kbn?: number;
  payee_name_disp_kbn_name?: string;
  payee_name_for_post_disp_kbn?: number;
  payee_name_for_post_disp_kbn_name?: string;
  payee_contact_id?: number | null;
  payee_contact?: Record<string, unknown> | null;
  company_branch_id?: number | null;
  company_branch?: CompanyBranchRef | null;
  company_name_disp_kbn?: number;
  company_name_disp_kbn_name?: string;
  company_name_for_post_disp_kbn?: number;
  company_name_for_post_disp_kbn_name?: string;
  user_id?: number | null;
  user?: UserRef | null;
  project_id?: number | null;
  project?: ProjectRef | null;
  invoice_dates?: string[];
  order_date?: string | null;
  payment_term_id?: number | null;
  payment_term_name?: string | null;
  /** 発注ステータス: 1 見積中(高) / 2 見積中(中) / 3 見積中(低) / 8 見積中(除) / 4 発注確定 / 5 発注済 / 9 見送り */
  expenditure_status: number;
  expenditure_status_name: string;
  payment_timing_kbn?: number;
  payment_timing_kbn_name?: string;
  tax_withholding_kbn?: number;
  tax_withholding_kbn_name?: string;
  in_house_memo?: string | null;
  management_no?: string | null;
  expenditure_type_id?: number | null;
  expenditure_type_name?: string | null;
  expenditure_type2_id?: number | null;
  expenditure_type2_name?: string | null;
  expenditure_type3_id?: number | null;
  expenditure_type3_name?: string | null;
  tags?: string[];
  group_id?: number | null;
  group_name?: string | null;
  accounting_type_id?: number | null;
  accounting_type_name?: string | null;
  accounting_type2_id?: number | null;
  accounting_type2_name?: string | null;
  accounting_type3_id?: number | null;
  accounting_type3_name?: string | null;
  company_bank_id?: number | null;
  /** 進捗状況: 1 未着手 / 2 着手中 / 3 納品済 / 4 検収済 */
  delivery_status: number;
  delivery_status_name: string;
  payment_method_kbn?: number;
  payment_method_kbn_name?: string;
  tax_rule_kbn?: number;
  tax_rule_kbn_name?: string;
  minus_rounding_rule?: number;
  minus_rounding_rule_name?: string;
  lock_flg?: number;
  wareki_flg?: number;
  archive_flg?: number;
  reduced_tax_rate_kbn?: number;
  document_setting_id?: number | null;
  document_setting_name?: string | null;
  to?: string | null;
  cc?: string | null;
  currency?: string;
  lang_kbn?: number;
  lang_kbn_name?: string;
  exchange_rate?: string | null;
  total_jpy?: number;
  tax_jpy?: number;
  created_at: string;
  updated_at: string;
  invoice_date?: string | null;
  total?: string | null;
  tax?: string | null;
  tax_withholding?: number;
  contract_start_date?: string | null;
  contract_end_date?: string | null;
  periodical_payment_interval?: number | null;
  periodical_payment_payment_kbn?: number | null;
  periodical_payment_payment_kbn_name?: string | null;
  auto_renewal_flg?: number;
  auto_renewal_period_month?: number | null;
  delivery_document_kbn?: number;
  /** response_group で書類を付けたときだけ入る */
  invoices?: unknown[];
  expenditure_estimate?: unknown;
  expenditure_order?: unknown;
  expenditure_deliveries?: unknown[];
  expenditure_invoices?: unknown[];
}

/** 発注ステータス(数値コード → 名称)。`expenditure_status_in[]` に送る値。 */
export const EXPENDITURE_STATUSES = {
  1: "見積中(高)",
  2: "見積中(中)",
  3: "見積中(低)",
  8: "見積中(除)",
  4: "発注確定",
  5: "発注済",
  9: "見送り",
} as const;

/** 進捗状況(数値コード → 名称)。`delivery_status_in[]` に送る値。 */
export const EXPENDITURE_DELIVERY_STATUSES = {
  1: "未着手",
  2: "着手中",
  3: "納品済",
  4: "検収済",
} as const;

/** 一覧の response_group を指定しないときに送る値(案件 `project` を含む)。 */
export const PURCHASE_ORDER_DEFAULT_RESPONSE_GROUP = "medium";

export interface PurchaseOrderListOptions extends CommonListOptions {
  /** 支払先 ID の完全一致(payee_id_eq) */
  payeeIdEq?: number;
  /** 案件 ID の完全一致(project_id_eq)。サーバー専用(キャッシュでは絞らない) */
  projectIdEq?: number;
  /** 発注ステータス(expenditure_status_in[]) */
  expenditureStatusIn?: number[];
  /** 進捗状況(delivery_status_in[]) */
  deliveryStatusIn?: number[];
  /** "small" / "medium" / "large"。未指定なら medium を送る */
  responseGroup?: string;
}

export function buildPurchaseOrdersQuery(
  opts: PurchaseOrderListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("purchase_orders", opts, page, perPage)
    .intEq("payee_id", opts.payeeIdEq)
    .intEq("project_id", opts.projectIdEq)
    .intIn("expenditure_status", opts.expenditureStatusIn)
    .intIn("delivery_status", opts.deliveryStatusIn)
    .responseGroup(opts.responseGroup || PURCHASE_ORDER_DEFAULT_RESPONSE_GROUP)
    .encode();
}

export function listPurchaseOrders(
  client: BoardClient,
  opts: PurchaseOrderListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<PurchaseOrderEntity>> {
  return listResource<PurchaseOrderEntity, PurchaseOrderListOptions>(
    client,
    "/v1/expenditures",
    buildPurchaseOrdersQuery,
    opts,
    paging,
    ctx,
  );
}

export interface GetPurchaseOrderOptions {
  /** "small"(BOARD の既定)/ "medium" / "large" / "estimate" / "order" / "delivery" / "invoice" / "all"。空なら付けない。 */
  responseGroup?: string;
}

export function getPurchaseOrder(
  client: BoardClient,
  id: number,
  opts: GetPurchaseOrderOptions = {},
  ctx?: CallContext,
): Promise<ItemResult<PurchaseOrderEntity>> {
  const query = new QueryBuilder().responseGroup(opts.responseGroup).encode();
  return getResource<PurchaseOrderEntity>(
    client,
    "/v1/expenditures",
    id,
    query || undefined,
    "id",
    ctx,
  );
}
