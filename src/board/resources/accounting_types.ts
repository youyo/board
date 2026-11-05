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

/** 経理種別。GET /v1/accounting_types の応答 1 要素。 */
export interface AccountingTypeEntity {
  id: number;
  name: string;
  memo: string;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
  [key: string]: unknown;
}

/** GET /v1/accounting_types のクエリ(Ransack)。未指定の項目は送らない。 */
export interface AccountingTypeListOptions extends CommonListOptions {
  /** 経理種別名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildAccountingTypesQuery(
  opts: AccountingTypeListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listAccountingTypes(
  client: BoardClient,
  opts: AccountingTypeListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<AccountingTypeEntity>> {
  return listResource<AccountingTypeEntity, AccountingTypeListOptions>(
    client,
    "/v1/accounting_types",
    buildAccountingTypesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getAccountingType(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<AccountingTypeEntity>> {
  return getResource<AccountingTypeEntity>(
    client,
    "/v1/accounting_types",
    id,
    undefined,
    "id",
    ctx,
  );
}
