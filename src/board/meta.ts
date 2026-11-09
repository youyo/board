/**
 * 応答ヘッダーから作る `_meta`(`show_meta` 用。Go 版 `result.go` の ListMeta / ItemMeta)。
 * キーは snake_case。0・空の値は出さない(Go の omitempty)。
 */

export interface ItemMeta {
  etag?: string;
  last_modified?: string;
}

export interface ListMeta extends ItemMeta {
  total_count?: number;
  page?: number;
  per_page?: number;
  /** 秒(429 / 503 のとき)。 */
  retry_after?: number;
}

/** Go の strconv.Atoi 相当(整数でなければ 0)。 */
function atoi(h: Headers, name: string): number {
  const v = h.get(name)?.trim();
  return v && /^[+-]?\d+$/.test(v) ? Number(v) : 0;
}

function put<K extends keyof ListMeta>(meta: ListMeta, key: K, value: ListMeta[K]): void {
  if (value !== undefined && value !== 0 && value !== "") meta[key] = value;
}

/** X-Ratelimit-* は BOARD の OpenAPI(v1.9.0)に記載が無く、実 API も返さないので読まない(issue #33)。 */
function common(h: Headers, meta: ListMeta): void {
  put(meta, "etag", h.get("ETag") ?? undefined);
  put(meta, "last_modified", h.get("Last-Modified") ?? undefined);
}

/** 一覧の `_meta`。1 ページも取っていなければ(headers なし)空。 */
export function parseListMeta(h: Headers | undefined): ListMeta {
  const meta: ListMeta = {};
  if (!h) return meta;
  put(meta, "total_count", atoi(h, "X-Total-Count"));
  put(meta, "page", atoi(h, "X-Page"));
  put(meta, "per_page", atoi(h, "X-Per-Page"));
  common(h, meta);
  put(meta, "retry_after", atoi(h, "Retry-After"));
  return meta;
}

/** 単体取得の `_meta`(ページング関係と retry_after は含めない)。 */
export function parseItemMeta(h: Headers): ItemMeta {
  const meta: ItemMeta = {};
  common(h, meta);
  return meta;
}
