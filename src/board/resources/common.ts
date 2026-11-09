import { ArgumentError, BoardError } from "../../errors";
import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { type ItemMeta, type ListMeta, parseItemMeta, parseListMeta } from "../meta";
import { LIST_FILTER_SUPPORT } from "../openapi-support";
import { fetchAllPages } from "../paging";
import { QueryBuilder } from "../query";

/**
 * BOARD のエンティティ。名前を付けたフィールドは Go 版の構造体(実 API と突合済み)に対応する。
 * 未知のフィールドも捨てずに素通しするため index signature を持つ(応答は検証せず生データを返す)。
 */
export type Entity = { [key: string]: unknown };

/** 全 List 共通のオプション(Go の XxxListOptions の先頭部分)。 */
export interface CommonListOptions {
  /** 通常は使わない。ページングは自動で、page は 1 から順に上書きされる(Go と同じ)。 */
  page?: number;
  /** 1 ページの件数(既定 100)。 */
  perPage?: number;
  /** "YYYY-MM-DD HH:MM:SS" */
  updatedAtGteq?: string;
  updatedAtLteq?: string;
  /** undefined=送らない、true=1、false=0 */
  includeArchiveFlg?: boolean;
}

export interface ListResult<T> {
  items: T[];
  /** 最終ページの応答ヘッダーから作ったメタ情報(`show_meta` で出す)。 */
  _meta: ListMeta;
  /** ページ数の上限で止めた(続きがある可能性がある)。 */
  truncated: boolean;
}

export interface ItemResult<T> {
  item: T;
  _meta: ItemMeta;
}

export interface ListPaging {
  /** 辿るページ数の上限(既定 10)。 */
  maxPages?: number;
  /** 正の整数なら、この件数に達したページで巡回を止める(API 経路の limit)。 */
  limit?: number;
}

/**
 * 全 List 共通のクエリ(page / per_page)から始め、updated_at_gteq / updated_at_lteq / include_archive_flg は
 * そのリソースの一覧が仕様に持つものだけを付ける(`LIST_FILTER_SUPPORT`。OpenAPI から生成)。
 * 仕様に無いクエリは BOARD にエラーにならず無視されるので、送らない(issue #28)。
 */
export function baseQuery(
  resource: string,
  opts: CommonListOptions,
  page: number,
  perPage: number,
): QueryBuilder {
  const support = LIST_FILTER_SUPPORT[resource];
  if (!support) throw new Error(`no list filter support: ${resource}`);
  const q = new QueryBuilder().page(page, perPage);
  if (support.updatedAtGteq) q.dateGteq("updated_at", opts.updatedAtGteq);
  if (support.updatedAtLteq) q.dateLteq("updated_at", opts.updatedAtLteq);
  if (support.includeArchiveFlg) q.flg01("include_archive_flg", opts.includeArchiveFlg);
  return q;
}

/** 全ページを順に取って 1 つの配列にする。 */
export async function listResource<T, O extends CommonListOptions>(
  client: BoardClient,
  path: string,
  build: (opts: O, page: number, perPage: number) => string,
  opts: O,
  paging: ListPaging = {},
  ctx?: CallContext,
): Promise<ListResult<T>> {
  const all = await fetchAllPages<T>(
    async (page, perPage, shared) => {
      const res = await client.get(path, build(opts, page, perPage), shared);
      if (!Array.isArray(res.body)) {
        throw new BoardError(
          "api_error",
          "BOARD API returned an unexpected response (expected a JSON array).",
          false,
        );
      }
      return { items: res.body as T[], headers: res.headers };
    },
    { perPage: opts.perPage, maxPages: paging.maxPages, limit: paging.limit, ctx },
  );
  return { items: all.items, _meta: parseListMeta(all.headers), truncated: all.truncated };
}

/** id は 1 以上の安全な整数に限る(パスの組み立てに使うため)。 */
export function assertId(id: number, label = "id"): void {
  if (!Number.isSafeInteger(id) || id < 1) {
    throw new ArgumentError(`${label} must be a positive integer.`);
  }
}

/** `<path>/<id>` を 1 件取る。 */
export async function getResource<T>(
  client: BoardClient,
  path: string,
  id: number,
  query?: string,
  label = "id",
  ctx?: CallContext,
): Promise<ItemResult<T>> {
  assertId(id, label);
  const res = await client.get(`${path}/${id}`, query, ctx);
  if (typeof res.body !== "object" || res.body === null || Array.isArray(res.body)) {
    throw new BoardError(
      "api_error",
      "BOARD API returned an unexpected response (expected a JSON object).",
      false,
    );
  }
  return { item: res.body as T, _meta: parseItemMeta(res.headers) };
}
