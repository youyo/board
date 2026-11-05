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
 * 仕入先支社(BOARD API では payee_branches)。GET /v1/payee_branches の応答 1 要素(Go の VendorBranchEntity)。
 * `vendor` のキー名は Go 版でも実データでの確認が済んでいない(0 件のため)。未知のキーは素通しされる。
 */
export interface VendorBranchEntity {
  id: number;
  vendor: VendorRef | null;
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

export interface VendorBranchListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(payee_id_eq) */
  payeeIdEq?: number;
  /** 支社名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildVendorBranchesQuery(
  opts: VendorBranchListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage)
    .intEq("payee_id", opts.payeeIdEq)
    .strCont("name", opts.nameCont)
    .encode();
}

export function listVendorBranches(
  client: BoardClient,
  opts: VendorBranchListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<VendorBranchEntity>> {
  return listResource<VendorBranchEntity, VendorBranchListOptions>(
    client,
    "/v1/payee_branches",
    buildVendorBranchesQuery,
    opts,
    paging,
  );
}

export function getVendorBranch(
  client: BoardClient,
  id: number,
): Promise<ItemResult<VendorBranchEntity>> {
  return getResource<VendorBranchEntity>(client, "/v1/payee_branches", id);
}
