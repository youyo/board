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
import type { VendorRef } from "./refs";

/**
 * 仕入先担当者(BOARD API では payee_contacts)。GET /v1/payee_contacts の応答 1 要素。
 * 仕入先の入れ子は `payee`(Go 版の `vendor` は仕様に無い。issue #28)。
 */
export interface VendorContactEntity {
  id: number;
  payee_id?: number;
  payee: VendorRef | null;
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
}

export interface VendorContactListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(payee_id_eq) */
  payeeIdEq?: number;
}

export function buildVendorContactsQuery(
  opts: VendorContactListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("vendor_contacts", opts, page, perPage)
    .intEq("payee_id", opts.payeeIdEq)
    .encode();
}

export function listVendorContacts(
  client: BoardClient,
  opts: VendorContactListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<VendorContactEntity>> {
  return listResource<VendorContactEntity, VendorContactListOptions>(
    client,
    "/v1/payee_contacts",
    buildVendorContactsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getVendorContact(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<VendorContactEntity>> {
  return getResource<VendorContactEntity>(client, "/v1/payee_contacts", id, undefined, "id", ctx);
}
