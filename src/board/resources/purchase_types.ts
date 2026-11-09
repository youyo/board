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

/** 仕入種別。GET /v1/expenditure_types の応答 1 要素。 */
export interface PurchaseTypeEntity {
  id: number;
  name: string;
  archive_flg?: number;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
}

/** GET /v1/expenditure_types のクエリ(Ransack)。未指定の項目は送らない。 */
export interface PurchaseTypeListOptions extends CommonListOptions {
  /** 仕入種別名の部分一致(name_cont) */
  nameCont?: string;
  /** 発注区分1〜3の種類(1〜3。expenditure_type_kbn_eq)。未指定は 1 と同じ結果。サーバー専用(キャッシュでは絞らない) */
  expenditureTypeKbnEq?: number;
}

export function buildPurchaseTypesQuery(
  opts: PurchaseTypeListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("purchase_types", opts, page, perPage)
    .strCont("name", opts.nameCont)
    .intEq("expenditure_type_kbn", opts.expenditureTypeKbnEq)
    .encode();
}

export function listPurchaseTypes(
  client: BoardClient,
  opts: PurchaseTypeListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<PurchaseTypeEntity>> {
  return listResource<PurchaseTypeEntity, PurchaseTypeListOptions>(
    client,
    "/v1/expenditure_types",
    buildPurchaseTypesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getPurchaseType(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<PurchaseTypeEntity>> {
  return getResource<PurchaseTypeEntity>(client, "/v1/expenditure_types", id, undefined, "id", ctx);
}
