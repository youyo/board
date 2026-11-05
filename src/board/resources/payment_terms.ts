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

/** 支払条件。GET /v1/payment_terms の応答 1 要素。 */
export interface PaymentTermEntity {
  id: number;
  name: string;
  memo: string;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
  [key: string]: unknown;
}

/** GET /v1/payment_terms のクエリ(Ransack)。未指定の項目は送らない。 */
export interface PaymentTermListOptions extends CommonListOptions {
  /** 支払条件名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildPaymentTermsQuery(
  opts: PaymentTermListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listPaymentTerms(
  client: BoardClient,
  opts: PaymentTermListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<PaymentTermEntity>> {
  return listResource<PaymentTermEntity, PaymentTermListOptions>(
    client,
    "/v1/payment_terms",
    buildPaymentTermsQuery,
    opts,
    paging,
  );
}

export function getPaymentTerm(
  client: BoardClient,
  id: number,
): Promise<ItemResult<PaymentTermEntity>> {
  return getResource<PaymentTermEntity>(client, "/v1/payment_terms", id);
}
