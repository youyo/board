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
import type { CompanyBranchRef, ProjectRef, UserRef, VendorRef } from "./refs";

/**
 * 支払(BOARD API では expenditure_payments)。GET /v1/expenditure_payments の応答 1 要素。
 * BOARD の OpenAPI(v1.9.0)の応答項目だけを持つ。支払先は入れ子の `payee`、発注は `expenditure_id` / `expenditure_no`、
 * ステータスは数値の `payment_status`。`vendor_id` / `purchase_order_id` / `status` は無い(issue #27)。
 */
export interface PaymentEntity {
  id: number;
  /** 発注の ID */
  expenditure_id: number;
  /** 発注 No */
  expenditure_no: number;
  management_no?: string | null;
  name: string;
  payee: VendorRef | null;
  payee_branch?: Record<string, unknown> | null;
  payee_contact?: Record<string, unknown> | null;
  company_branch?: CompanyBranchRef | null;
  user?: UserRef | null;
  total?: string | null;
  tax?: string | null;
  tax_withholding?: number;
  project?: ProjectRef | null;
  invoice_date?: string | null;
  payment_date?: string | null;
  /** 発注ステータス(発注書の expenditure_status と同じコード) */
  expenditure_status: number;
  expenditure_status_name: string;
  /** 支払ステータス: 1 請求書未受領 / 2 請求書受領済 / 4 振込予約済 / 3 支払済 */
  payment_status: number;
  payment_status_name: string;
  payment_method_kbn?: number;
  payment_method_kbn_name?: string;
  expenditure_type_id?: number | null;
  expenditure_type_name?: string | null;
  expenditure_type2_id?: number | null;
  expenditure_type2_name?: string | null;
  expenditure_type3_id?: number | null;
  expenditure_type3_name?: string | null;
  group_id?: number | null;
  group_name?: string | null;
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
}

/** 支払ステータス(数値コード → 名称)。`payment_status_in[]` に送る値。 */
export const PAYMENT_STATUSES = {
  1: "請求書未受領",
  2: "請求書受領済",
  4: "振込予約済",
  3: "支払済",
} as const;

export interface PaymentListOptions extends CommonListOptions {
  /** 支払先 ID の完全一致(expenditure_payee_id_eq) */
  expenditurePayeeIdEq?: number;
  /** 支払ステータス(payment_status_in[]) */
  paymentStatusIn?: number[];
  /** 発注 No の完全一致(expenditure_expenditure_no_eq)。発注 ID ではない */
  expenditureExpenditureNoEq?: number;
  /** "small" / "large" */
  responseGroup?: string;
}

export function buildPaymentsQuery(
  opts: PaymentListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("expenditure_payee_id", opts.expenditurePayeeIdEq)
    .intIn("payment_status", opts.paymentStatusIn)
    .intEq("expenditure_expenditure_no", opts.expenditureExpenditureNoEq)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listPayments(
  client: BoardClient,
  opts: PaymentListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<PaymentEntity>> {
  return listResource<PaymentEntity, PaymentListOptions>(
    client,
    "/v1/expenditure_payments",
    buildPaymentsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getPayment(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<PaymentEntity>> {
  return getResource<PaymentEntity>(client, "/v1/expenditure_payments", id, undefined, "id", ctx);
}
