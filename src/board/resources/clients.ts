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

/**
 * 顧客。GET /v1/clients の応答 1 要素(Go の ClientEntity)。
 * `response_group=large` または Get のときだけ返るフィールドは省略可能(?)にしてある。
 */
export interface ClientEntity {
  id: number;
  name: string;
  updated_at: string;
  created_at: string;
  name_disp: string;
  title: string | null;
  zip: string | null;
  pref: string | null;
  address1: string | null;
  address2: string | null;
  tel: string | null;
  fax: string | null;
  payment_term_id: number;
  payment_term_name: string;
  company_number: string | null;
  invoice_system_number: string | null;
  invoice_system_number_validated: boolean;
  invoice_system_issuer_type: number;
  invoice_system_issuer_type_name: string;
  // Get / large のみ
  accounting_code?: string | null;
  archive_flg?: number;
  bank_charge_to_client_flg?: number;
  basic_agreement_flg?: number;
  cc?: string | null;
  company_bank_id?: number | null;
  company_bank_name?: string | null;
  custom_no?: string | null;
  document_send_type?: number;
  document_send_type_name?: string;
  nda_flg?: number;
  note?: string | null;
  tags?: string[];
  to?: string | null;
  [key: string]: unknown;
}

/** GET /v1/clients のクエリ(Ransack)。未指定の項目は送らない。 */
export interface ClientListOptions extends CommonListOptions {
  /** 顧客名の部分一致(name_cont) */
  nameCont?: string;
  /** 顧客略称の部分一致(name_disp_cont) */
  nameDispCont?: string;
  /** 適格請求書登録番号の完全一致(invoice_system_number_eq) */
  invoiceSystemNumberEq?: string;
  /** 顧客番号の完全一致(custom_no_eq) */
  customNoEq?: string;
  /** タグ(tags[]) */
  tags?: string[];
  /** "small"(既定)/ "large"(Get 限定のフィールドも含む) */
  responseGroup?: string;
}

export function buildClientsQuery(opts: ClientListOptions, page: number, perPage: number): string {
  return baseQuery(opts, page, perPage)
    .strCont("name", opts.nameCont)
    .strCont("name_disp", opts.nameDispCont)
    .strEq("invoice_system_number", opts.invoiceSystemNumberEq)
    .strEq("custom_no", opts.customNoEq)
    .tags(opts.tags)
    .responseGroup(opts.responseGroup)
    .encode();
}

export function listClients(
  client: BoardClient,
  opts: ClientListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<ClientEntity>> {
  return listResource<ClientEntity, ClientListOptions>(
    client,
    "/v1/clients",
    buildClientsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getClient(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<ClientEntity>> {
  return getResource<ClientEntity>(client, "/v1/clients", id, undefined, "id", ctx);
}
