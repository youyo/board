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
 * 仕入先支社(BOARD API では payee_branches)。GET /v1/payee_branches の応答 1 要素。
 * 仕入先の入れ子は `payee`(Go 版の `vendor` は仕様に無い。issue #28)。`payee` の形は、仕様の schema が `{}` の箇所があり、
 * 実データでの確認が済んでいない(id / name / name_disp / custom_no は発注先の入れ子と同じ想定)。
 */
export interface VendorBranchEntity {
  id: number;
  payee_id?: number;
  payee: VendorRef | null;
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
}

export interface VendorBranchListOptions extends CommonListOptions {
  /** 仕入先 ID の完全一致(payee_id_eq) */
  payeeIdEq?: number;
}

export function buildVendorBranchesQuery(
  opts: VendorBranchListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("vendor_branches", opts, page, perPage)
    .intEq("payee_id", opts.payeeIdEq)
    .encode();
}

export function listVendorBranches(
  client: BoardClient,
  opts: VendorBranchListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<VendorBranchEntity>> {
  return listResource<VendorBranchEntity, VendorBranchListOptions>(
    client,
    "/v1/payee_branches",
    buildVendorBranchesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getVendorBranch(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<VendorBranchEntity>> {
  return getResource<VendorBranchEntity>(client, "/v1/payee_branches", id, undefined, "id", ctx);
}
