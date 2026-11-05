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
import type { ProjectCostEntity } from "./project_costs";
import type {
  ClientBranchRef,
  ClientRef,
  CompanyBranchRef,
  ContactRef,
  DocumentSummary,
  HubspotRef,
  UserRef,
} from "./refs";

/**
 * 案件。GET /v1/projects の応答 1 要素(Go の ProjectEntity)。
 * BOARD は金額を文字列("90000.0")で返す。estimate 以下は response_group を指定したときだけ入る。
 */
export interface ProjectEntity {
  id: number;
  name: string;
  updated_at: string;
  created_at: string;
  project_no: number | null;
  management_no: string | null;
  client: ClientRef | null;
  contact: ContactRef | null;
  user: UserRef | null;
  total: string | null;
  tax: string | null;
  estimate_date: string | null;
  invoice_dates: string[];
  order_status: number;
  order_status_name: string;
  delivery_status: number;
  delivery_status_name: string;
  project_type_id: number | null;
  project_type_name: string | null;
  project_type2_id: number | null;
  project_type2_name: string | null;
  project_type3_id: number | null;
  project_type3_name: string | null;
  group_id: number | null;
  group_name: string | null;
  client_branch: ClientBranchRef | null;
  company_branch: CompanyBranchRef | null;
  hubspot: HubspotRef | null;
  client_name_disp_kbn: number;
  client_name_disp_kbn_name: string;
  client_name_for_post_disp_kbn: number;
  client_name_for_post_disp_kbn_name: string;
  company_name_disp_kbn: number;
  company_name_disp_kbn_name: string;
  company_name_for_post_disp_kbn: number;
  company_name_for_post_disp_kbn_name: string;
  cost_total: number;
  cost_tax: number;
  invoice_total: string | null;
  invoice_tax: string | null;
  delivery_date: string | null;
  delivery_date_text: string | null;
  ordered_date: string | null;
  payment_term_id: number | null;
  payment_term_name: string | null;
  payment_method_kbn: number;
  payment_method_kbn_name: string;
  invoice_timing_kbn: number;
  invoice_timing_kbn_name: string;
  contract_start_date: string | null;
  contract_end_date: string | null;
  periodical_invoice_interval: number | null;
  periodical_invoice_payment_kbn: number | null;
  contract_end_alert_flg: number;
  auto_renewal_flg: number;
  auto_renewal_period_month: number | null;
  monthly_invoice_payment_kbn: number | null;
  delivery_document_kbn: number;
  accounting_type_id: number | null;
  accounting_type_name: string | null;
  accounting_type2_id: number | null;
  accounting_type2_name: string | null;
  accounting_type3_id: number | null;
  accounting_type3_name: string | null;
  tags: string[];
  in_house_memo: string | null;
  to: string | null;
  cc: string | null;
  lock_flg: number;
  archive_flg: number;
  document_setting_id: number | null;
  document_setting_name: string | null;
  // response_group 指定時のみ
  estimate?: DocumentSummary | null;
  order?: DocumentSummary | null;
  deliveries?: DocumentSummary[];
  invoices?: DocumentSummary[];
  receipts?: DocumentSummary[];
  project_costs?: ProjectCostEntity[];
  [key: string]: unknown;
}

/** GET /v1/projects のクエリ(Ransack)。未指定の項目は送らない。 */
export interface ProjectListOptions extends CommonListOptions {
  /** "YYYY-MM-DD HH:MM:SS" */
  createdAtGteq?: string;
  createdAtLteq?: string;
  /** 失注案件を含める(include_lost_flg)。undefined=送らない */
  includeLostFlg?: boolean;
  /** 案件名の部分一致(name_cont) */
  nameCont?: string;
  /** 顧客 ID の完全一致(client_id_eq) */
  clientIdEq?: number;
  /** 顧客名の部分一致(client_name_cont) */
  clientNameCont?: string;
  /** 受注ステータス(order_status_in[]) */
  orderStatusIn?: number[];
  /** 納品ステータス(delivery_status_in[]) */
  deliveryStatusIn?: number[];
  /** 案件番号の完全一致(project_no_eq) */
  projectNoEq?: string;
  /** 管理番号の完全一致(management_no_eq) */
  managementNoEq?: string;
  /** 納期 >= / <=(YYYY-MM-DD) */
  deliveryDateGteq?: string;
  deliveryDateLteq?: string;
  /** 請求日 >= / <=(YYYY-MM-DD) */
  invoiceDateGteq?: string;
  invoiceDateLteq?: string;
  /** 請求タイミング区分(invoice_timing_kbn_in[]) */
  invoiceTimingKbnIn?: number[];
  tags?: string[];
  /** "small" / "large" / "estimate" / "order" / "delivery" / "invoice" / "receipt" / "all" */
  responseGroup?: string;
}

export function buildProjectsQuery(
  opts: ProjectListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .strCont("name", opts.nameCont)
    .intEq("client_id", opts.clientIdEq)
    .strCont("client_name", opts.clientNameCont)
    .intIn("order_status", opts.orderStatusIn)
    .intIn("delivery_status", opts.deliveryStatusIn)
    .strEq("project_no", opts.projectNoEq)
    .strEq("management_no", opts.managementNoEq)
    .dateGteq("delivery_date", opts.deliveryDateGteq)
    .dateLteq("delivery_date", opts.deliveryDateLteq)
    .dateGteq("invoice_date", opts.invoiceDateGteq)
    .dateLteq("invoice_date", opts.invoiceDateLteq)
    .intIn("invoice_timing_kbn", opts.invoiceTimingKbnIn)
    .dateGteq("created_at", opts.createdAtGteq)
    .dateLteq("created_at", opts.createdAtLteq)
    .flg01("include_lost_flg", opts.includeLostFlg)
    .tags(opts.tags)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listProjects(
  client: BoardClient,
  opts: ProjectListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<ProjectEntity>> {
  return listResource<ProjectEntity, ProjectListOptions>(
    client,
    "/v1/projects",
    buildProjectsQuery,
    opts,
    paging,
    ctx,
  );
}

export interface GetProjectOptions {
  /** "estimate" / "order" / "delivery" / "invoice" / "receipt" / "all"。空なら付けない。 */
  responseGroup?: string;
}

export function getProject(
  client: BoardClient,
  id: number,
  opts: GetProjectOptions = {},
  ctx?: CallContext,
): Promise<ItemResult<ProjectEntity>> {
  const query = new QueryBuilder().responseGroup(opts.responseGroup).encode();
  return getResource<ProjectEntity>(client, "/v1/projects", id, query || undefined, "id", ctx);
}
