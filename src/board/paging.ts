/**
 * 全件取得のページング(Go 版 `internal/boardapi/pagination.go` の移植)。
 *
 * `page` / `per_page` で順番に取り(並列にしない)、件数が `per_page` 未満のページで終わる。
 * Go との違い: ページ数の上限 `maxPages` を必ず付け、上限で止めたときは `truncated: true` を返す。
 */

import { ArgumentError } from "../errors";
import { type CallContext, resolveContext } from "./context";

/** 1 ページの件数の既定(Go の defaultPerPage)。 */
export const DEFAULT_PER_PAGE = 100;
/** 辿るページ数の上限の既定。per_page 100 で最大 1,000 件。 */
export const DEFAULT_MAX_PAGES = 10;

export interface PagingOptions {
  /** 0 以下は未指定扱い(既定 100)。 */
  perPage?: number;
  /** 正の整数。既定 10。 */
  maxPages?: number;
  /**
   * 正の整数なら、集めた件数がこれに達したページで巡回を止める(truncated は false)。
   * perPage 未指定のときは「達したかどうか」だけでなく「超える分があるか」も分かるよう min(limit + 1, 100) にする。
   */
  limit?: number;
  /** 呼び出しコンテキスト。全ページで同じ締め切り・signal を共有する(deadline 省略時は開始時点で決める)。 */
  ctx?: CallContext;
}

export interface Page<T> {
  items: T[];
  headers: Headers;
}

export interface AllPages<T> {
  items: T[];
  /** 最後に取ったページの応答ヘッダー(`_meta` の元)。1 ページも取っていなければ undefined。 */
  headers: Headers | undefined;
  /** maxPages で止めた(続きがある可能性がある)。 */
  truncated: boolean;
  /** 取得したページ数。 */
  pages: number;
}

export async function fetchAllPages<T>(
  fetchPage: (page: number, perPage: number, ctx: CallContext) => Promise<Page<T>>,
  options: PagingOptions = {},
): Promise<AllPages<T>> {
  const limit = options.limit !== undefined && options.limit > 0 ? options.limit : undefined;
  const perPage = resolvePerPage(options.perPage, limit);
  const maxPages = options.maxPages ?? DEFAULT_MAX_PAGES;
  if (!Number.isInteger(maxPages) || maxPages < 1) {
    throw new ArgumentError("maxPages must be a positive integer.");
  }

  const ctx = resolveContext(options.ctx);
  const items: T[] = [];
  let headers: Headers | undefined;
  for (let page = 1; page <= maxPages; page++) {
    const r = await fetchPage(page, perPage, ctx);
    for (const item of r.items) items.push(item);
    headers = r.headers;
    if (limit !== undefined && items.length >= limit) {
      return { items, headers, truncated: false, pages: page };
    }
    if (r.items.length < perPage) return { items, headers, truncated: false, pages: page };
  }
  // 最後のページが満杯のまま上限に達した: 続きがあるかもしれない。
  return { items, headers, truncated: true, pages: maxPages };
}

function resolvePerPage(perPage: number | undefined, limit?: number): number {
  if (perPage === undefined || perPage <= 0) {
    return limit === undefined ? DEFAULT_PER_PAGE : Math.min(limit + 1, DEFAULT_PER_PAGE);
  }
  if (!Number.isInteger(perPage)) throw new ArgumentError("perPage must be an integer.");
  return perPage;
}
