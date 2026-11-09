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
 * 仕入先(BOARD API では payees)。GET /v1/payees の応答 1 要素。
 * OpenAPI(v1.9.0)の応答項目だけを持つ。Go 版の `code` / `memo` は仕様に無い(issue #28)。
 * `response_group=large` または Get のときだけ返る項目は省略可能(?)。
 */
export interface VendorEntity {
  id: number;
  name: string;
  name_disp?: string;
  updated_at: string;
  created_at: string;
  custom_no?: string | null;
  archive_flg?: number;
  tags?: string[];
}

export interface VendorListOptions extends CommonListOptions {
  /** 仕入先名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildVendorsQuery(opts: VendorListOptions, page: number, perPage: number): string {
  return baseQuery("vendors", opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listVendors(
  client: BoardClient,
  opts: VendorListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<VendorEntity>> {
  return listResource<VendorEntity, VendorListOptions>(
    client,
    "/v1/payees",
    buildVendorsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getVendor(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<VendorEntity>> {
  return getResource<VendorEntity>(client, "/v1/payees", id, undefined, "id", ctx);
}
