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

/** 仕入種別。GET /v1/expenditure_types の応答 1 要素。 */
export interface PurchaseTypeEntity {
  id: number;
  name: string;
  memo: string;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
  [key: string]: unknown;
}

/** GET /v1/expenditure_types のクエリ(Ransack)。未指定の項目は送らない。 */
export interface PurchaseTypeListOptions extends CommonListOptions {
  /** 仕入種別名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildPurchaseTypesQuery(
  opts: PurchaseTypeListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listPurchaseTypes(
  client: BoardClient,
  opts: PurchaseTypeListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<PurchaseTypeEntity>> {
  return listResource<PurchaseTypeEntity, PurchaseTypeListOptions>(
    client,
    "/v1/expenditure_types",
    buildPurchaseTypesQuery,
    opts,
    paging,
  );
}

export function getPurchaseType(
  client: BoardClient,
  id: number,
): Promise<ItemResult<PurchaseTypeEntity>> {
  return getResource<PurchaseTypeEntity>(client, "/v1/expenditure_types", id);
}
