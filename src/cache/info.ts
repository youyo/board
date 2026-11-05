/**
 * 応答に同梱するキャッシュの鮮度情報(Go 版 `internal/cache/info.go`)。
 * LLM が `cached_at` / `full_refreshed_at` を見て、次の呼び出しで refresh=delta / full を指示するための材料。
 * 未取得なら cached_at / full_refreshed_at を出さない(LLM が『未取得』と判別できる)。
 */

import { ensureSchema } from "./schema";
import { getSyncState, type SyncState } from "./sync-state";

export type InfoRefreshState = "complete" | "partial" | "failed";

export interface CacheInfo {
  resource: string;
  /** 最終同期(delta または full の完了)。ISO 8601 UTC。 */
  cached_at?: string;
  /** 最終 full refresh の完了。削除されたエンティティはこれが新しいときだけ反映されている。 */
  full_refreshed_at?: string;
  /** complete / partial(full が途中。refresh=full で続きを取る)/ failed(delta が失敗し、古いキャッシュを返している)。 */
  refresh_state?: InfoRefreshState;
  /** failed のときのエラーコード(メッセージは載せない)。 */
  refresh_error_code?: string;
}

export interface InfoOutcome {
  state: InfoRefreshState;
  errorCode?: string;
}

export function cacheInfoFromState(
  resource: string,
  state: SyncState | null,
  outcome?: InfoOutcome,
): CacheInfo {
  const info: CacheInfo = { resource };
  if (state?.last_synced_at) info.cached_at = state.last_synced_at;
  if (state?.last_full_synced_at) info.full_refreshed_at = state.last_full_synced_at;
  const refreshState = outcome?.state ?? (state?.full_next_page != null ? "partial" : undefined);
  if (refreshState) info.refresh_state = refreshState;
  if (outcome?.state === "failed" && outcome.errorCode) info.refresh_error_code = outcome.errorCode;
  return info;
}

/** 指定したリソースの鮮度情報を sync_state から読む。要素は指定順で、常にリソースの数だけ返す。 */
export async function loadCacheInfos(
  db: D1Database,
  resources: readonly string[],
  outcomes: ReadonlyMap<string, InfoOutcome> = new Map(),
): Promise<CacheInfo[]> {
  await ensureSchema(db);
  const infos: CacheInfo[] = [];
  for (const r of resources) {
    infos.push(cacheInfoFromState(r, await getSyncState(db, r), outcomes.get(r)));
  }
  return infos;
}
