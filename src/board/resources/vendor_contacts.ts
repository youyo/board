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
import type { VendorRef } from "./refs";

/**
 * 仕入先担当者(BOARD API では payee_contacts)。GET /v1/payee_contacts の応答 1 要素(Go の VendorContactEntity)。
 * `vendor` のキー名は Go 版でも実データでの確認が済んでいない(0 件のため)。未知のキーは素通しされる。
 */
export interface VendorContactEntity {
  id: number;
  vendor: VendorRef | null;
  last_name: string;
  first_name: string;
  honorific_title: string;
  title: string | null;
  department: string | null;
  email: string | null;
  note: string | null;
  archive_flg: number;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface VendorContactListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(payee_id_eq) */
  payeeIdEq?: number;
  /** 担当者名の部分一致(name_cont) */
  nameCont?: string;
  /** メールアドレスの部分一致(email_cont) */
  emailCont?: string;
}

export function buildVendorContactsQuery(
  opts: VendorContactListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("payee_id", opts.payeeIdEq)
    .strCont("name", opts.nameCont)
    .strCont("email", opts.emailCont)
    .encode();
}

export function listVendorContacts(
  client: BoardClient,
  opts: VendorContactListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<VendorContactEntity>> {
  return listResource<VendorContactEntity, VendorContactListOptions>(
    client,
    "/v1/payee_contacts",
    buildVendorContactsQuery,
    opts,
    paging,
  );
}

export function getVendorContact(
  client: BoardClient,
  id: number,
): Promise<ItemResult<VendorContactEntity>> {
  return getResource<VendorContactEntity>(client, "/v1/payee_contacts", id);
}
