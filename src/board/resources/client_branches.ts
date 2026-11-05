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

/** 顧客支社。GET /v1/client_branches の応答 1 要素(Go の ClientBranchEntity)。 */
export interface ClientBranchEntity {
  id: number;
  client: ClientRef | null;
  name: string;
  zip: string;
  pref: string;
  address1: string;
  address2: string;
  tel: string | null;
  fax: string | null;
  archive_flg: number;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface ClientBranchListOptions extends CommonListOptions {
  /** 顧客 ID の完全一致(client_id_eq) */
  clientIdEq?: number;
  /** 支社名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildClientBranchesQuery(
  opts: ClientBranchListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("client_id", opts.clientIdEq)
    .strCont("name", opts.nameCont)
    .encode();
}

export function listClientBranches(
  client: BoardClient,
  opts: ClientBranchListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<ClientBranchEntity>> {
  return listResource<ClientBranchEntity, ClientBranchListOptions>(
    client,
    "/v1/client_branches",
    buildClientBranchesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getClientBranch(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<ClientBranchEntity>> {
  return getResource<ClientBranchEntity>(client, "/v1/client_branches", id, undefined, "id", ctx);
}
