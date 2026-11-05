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

/** 仕入先(BOARD API では payees)。GET /v1/payees の応答 1 要素(Go の VendorEntity)。 */
export interface VendorEntity {
  id: number;
  name: string;
  code: string;
  memo: string;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface VendorListOptions extends CommonListOptions {
  /** 仕入先名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildVendorsQuery(opts: VendorListOptions, page: number, perPage: number): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listVendors(
  client: BoardClient,
  opts: VendorListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<VendorEntity>> {
  return listResource<VendorEntity, VendorListOptions>(
    client,
    "/v1/payees",
    buildVendorsQuery,
    opts,
    paging,
  );
}

export function getVendor(client: BoardClient, id: number): Promise<ItemResult<VendorEntity>> {
  return getResource<VendorEntity>(client, "/v1/payees", id);
}
