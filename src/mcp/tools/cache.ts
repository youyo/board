/**
 * cache 系のツール 2 個(M06)。Go 版 CLI の `cache status / clear` に相当する。
 * `cache_expire` は作らない: Go では daily auto-refresh の廃止後、expire の印は読み取りに影響しなくなった
 * (情報用の列に印を付けるだけ)。LLM に渡しても何も変わらないので Workers 版には移さない(計画書 M06 の『Go との相違点』)。
 */
import { z } from "zod";
import { cacheReport, clearCache, clearDocumentMap, RESOURCE_NAMES } from "../../cache/admin";
import { RefreshInProgressError } from "../../errors";
import { optBool } from "../args";
import { defineTool } from "../registry";

export const cacheStatus = defineTool({
  name: "cache_status",
  title: "キャッシュ状態",
  description:
    "Show the state of the local D1 cache for all 22 BOARD resources, without calling the BOARD API. " +
    "Use it to decide whether to pass refresh=delta or refresh=full to other tools, or to see how far a long refresh has progressed. " +
    "resources[]: {resource, cached_at, full_refreshed_at, refresh_state, row_count, full_prune_skipped, locked}. " +
    "cached_at / full_refreshed_at are absent when never fetched. " +
    "refresh_state: complete = up to date as of cached_at; partial = a full refresh is unfinished (call a tool with refresh=full again until it is complete); " +
    "failed = the last refresh failed (last_error_code); not_synced = never fetched (cache is empty, so tools return nothing until refreshed). " +
    "full_prune_skipped=true means the last full refresh could not confirm the total count and kept old rows (run refresh=full again). " +
    "locked=true means a refresh is running now (concurrent refreshes return refresh_in_progress). " +
    "document_scans[]: progress of the document-to-project reverse lookup per document kind (estimate/order/delivery/receipt): " +
    "scan_state (not_started/in_progress/complete), mapped_rows, next_page.",
  category: "ReadOnly",
  inputSchema: z.strictObject({}),
  handler: async (_args, ctx) => cacheReport(ctx.db(), { now: () => ctx.now().getTime() }),
});

const RESOURCES_DESC = `Resources to clear (default: all 22). Valid: ${RESOURCE_NAMES.join(", ")}.`;
const DOCUMENT_MAP_DESC =
  "Also delete the document-to-project reverse lookup (default false). The mapping never changes, so keep it unless it looks wrong; it is rebuilt by find_* tools and scans.";

export const cacheClear = defineTool({
  name: "cache_clear",
  title: "キャッシュ削除",
  description:
    "Destructive: delete the cached rows and sync state of the given resources (all 22 if resources is omitted) from the D1 cache. " +
    "Use it only when the cache looks corrupt or you must drop everything and start over; to just get fresher data, pass refresh=delta or refresh=full to a read tool instead. " +
    "After clearing, the next read of that resource returns nothing until a refresh (refresh=full) rebuilds it, which may take several calls for large resources. " +
    "If a refresh is running for any target, nothing is deleted and refresh_in_progress is returned (retry after retry_after_seconds). " +
    "The document-to-project reverse lookup is kept unless include_document_map=true. Does not call the BOARD API.",
  category: "Destructive",
  inputSchema: z.strictObject({
    // 空配列を「全件」と取り違えないよう、指定するなら 1 件以上に限る(省略時だけが全件)。
    resources: z.array(z.string()).min(1).describe(RESOURCES_DESC).optional(),
    include_document_map: optBool(DOCUMENT_MAP_DESC),
  }),
  handler: async ({ resources, include_document_map }, ctx) => {
    const db = ctx.db();
    const now = () => ctx.now().getTime();
    const { cleared } = await clearCache(db, { resources, now });
    if (!include_document_map) return { cleared, document_map_cleared: false };
    try {
      await clearDocumentMap(db, { now });
    } catch (e) {
      if (!(e instanceof RefreshInProgressError)) throw e;
      // リソースは消えている。逆引きだけが走査中で消せなかったことを伝える(再実行すれば消える)。
      return {
        cleared,
        document_map_cleared: false,
        document_map_error: {
          code: e.code,
          message: e.message,
          retryable: true,
          retry_after_seconds: e.retryAfterSeconds,
        },
      };
    }
    return { cleared, document_map_cleared: true };
  },
});

export const CACHE_TOOLS = [cacheStatus, cacheClear];
