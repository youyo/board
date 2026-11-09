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
import type { ClientRef } from "./refs";

/** 担当者。GET /v1/contacts の応答 1 要素(Go の ContactEntity)。 */
export interface ContactEntity {
  id: number;
  client_id?: number;
  client: ClientRef | null;
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

export interface ContactListOptions extends CommonListOptions {
  /** 顧客 ID の完全一致(client_id_eq) */
  clientIdEq?: number;
}

export function buildContactsQuery(
  opts: ContactListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("contacts", opts, page, perPage).intEq("client_id", opts.clientIdEq).encode();
}

export function listContacts(
  client: BoardClient,
  opts: ContactListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<ContactEntity>> {
  return listResource<ContactEntity, ContactListOptions>(
    client,
    "/v1/contacts",
    buildContactsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getContact(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<ContactEntity>> {
  return getResource<ContactEntity>(client, "/v1/contacts", id, undefined, "id", ctx);
}
