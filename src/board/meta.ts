/**
 * 応答ヘッダーから作る `_meta`(`show_meta` 用。Go 版 `result.go` の ListMeta / ItemMeta)。
 * キーは snake_case。0・空の値は出さない(Go の omitempty)。
 */

export interface ItemMeta {
  rate_limit_remaining?: number;
  rate_limit_limit?: number;
  /** ISO 8601 (UTC、秒まで)。 */
  rate_limit_reset?: string;
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

function resetIso(h: Headers): string | undefined {
  const v = h.get("X-Ratelimit-Reset")?.trim();
  if (!v || !/^[+-]?\d+$/.test(v)) return undefined;
  const d = new Date(Number(v) * 1000);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString().replace(/\.\d{3}Z$/, "Z");
}

function put<K extends keyof ListMeta>(meta: ListMeta, key: K, value: ListMeta[K]): void {
  if (value !== undefined && value !== 0 && value !== "") meta[key] = value;
}

/** ヘッダー名の大文字小文字の揺れ(X-Ratelimit-* / X-RateLimit-*)は Headers が吸収する。 */
function common(h: Headers, meta: ListMeta): void {
  put(meta, "rate_limit_remaining", atoi(h, "X-Ratelimit-Remaining"));
  put(meta, "rate_limit_limit", atoi(h, "X-Ratelimit-Limit"));
  put(meta, "rate_limit_reset", resetIso(h));
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
