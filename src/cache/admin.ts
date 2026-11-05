/**
 * キャッシュの運用操作(Go 版 `internal/cli/cache.go` の status / expire / clear)。M06 の cache 系ツールの土台。
 * expire は Go と同じく sync_state に印を付けるだけで、読み取りの挙動は変えない(daily auto-refresh の廃止後は情報用)。
 */

import { RESOURCES } from "../board/resources";
import { ArgumentError, RefreshInProgressError } from "../errors";
import { LOCK_STALE_MS } from "../refresh/lock";
import { deleteAllEntries } from "./resource-cache";
import { ensureSchema } from "./schema";
import { deleteSyncStates, expireStates, listSyncStates } from "./sync-state";
import { type Clock, isoSeconds, systemClock } from "./time";

export const RESOURCE_NAMES: readonly string[] = RESOURCES.map((r) => r.name);

export interface CacheStatusRow {
  resource: string;
  cached_rows: number;
  last_synced_at?: string;
  last_full_synced_at?: string;
  last_sync_mode?: string;
  last_sync_status?: string;
  last_error_code?: string;
  expired_at?: string;
  must_full_resync: boolean;
  consecutive_errors: number;
  /** refresh が実行中(stale でないロックがある)。 */
  refreshing: boolean;
  /** full が途中(partial)。次の refresh=full は next_page から再開する。 */
  full_in_progress?: { generation: number; next_page: number; started_at?: string };
}

interface AdminOptions {
  now?: Clock;
}

function validate(resources: readonly string[] | undefined): readonly string[] {
  if (!resources || resources.length === 0) return RESOURCE_NAMES;
  const unknown = resources.filter((r) => !RESOURCE_NAMES.includes(r));
  if (unknown.length > 0) {
    throw new ArgumentError(
      `Unknown resource: ${unknown.join(", ")}. Valid resources: ${RESOURCE_NAMES.join(", ")}.`,
    );
  }
  return [...new Set(resources)];
}

export async function cacheStatus(
  db: D1Database,
  opts: AdminOptions = {},
): Promise<CacheStatusRow[]> {
  await ensureSchema(db);
  const now = (opts.now ?? systemClock)();
  const [states, counts] = await Promise.all([
    listSyncStates(db),
    db
      .prepare("SELECT resource_name, count(*) AS c FROM resource_cache GROUP BY resource_name")
      .all<{ resource_name: string; c: number }>(),
  ]);
  const byName = new Map(states.map((s) => [s.resource_name, s]));
  const rows = new Map(counts.results.map((r) => [r.resource_name, r.c]));
  return RESOURCE_NAMES.map((resource) => {
    const s = byName.get(resource);
    const started = s?.refresh_started_at ? Date.parse(s.refresh_started_at) : Number.NaN;
    const row: CacheStatusRow = {
      resource,
      cached_rows: rows.get(resource) ?? 0,
      must_full_resync: (s?.must_full_resync ?? 0) !== 0,
      consecutive_errors: s?.consecutive_failures ?? 0,
      refreshing: !Number.isNaN(started) && started >= now - LOCK_STALE_MS,
    };
    if (s?.last_synced_at) row.last_synced_at = s.last_synced_at;
    if (s?.last_full_synced_at) row.last_full_synced_at = s.last_full_synced_at;
    if (s?.last_sync_mode) row.last_sync_mode = s.last_sync_mode;
    if (s?.last_sync_status) row.last_sync_status = s.last_sync_status;
    if (s?.last_error_code) row.last_error_code = s.last_error_code;
    if (s?.expired_at) row.expired_at = s.expired_at;
    if (s?.full_next_page != null) {
      row.full_in_progress = {
        generation: s.full_generation,
        next_page: s.full_next_page,
        ...(s.full_started_at ? { started_at: s.full_started_at } : {}),
      };
    }
    return row;
  });
}

export async function expireCache(
  db: D1Database,
  opts: AdminOptions & { resources?: readonly string[] } = {},
): Promise<{ expired: readonly string[] }> {
  const targets = validate(opts.resources);
  await expireStates(db, targets, isoSeconds((opts.now ?? systemClock)()));
  return { expired: targets };
}

/** 行と同期状態を消す。refresh 中(ロック保持)のリソースがあれば、何も消さずに RefreshInProgressError。 */
export async function clearCache(
  db: D1Database,
  opts: AdminOptions & { resources?: readonly string[] } = {},
): Promise<{ cleared: readonly string[] }> {
  const targets = validate(opts.resources);
  const busy = (await cacheStatus(db, opts)).find(
    (r) => r.refreshing && targets.includes(r.resource),
  );
  if (busy) throw new RefreshInProgressError(busy.resource);
  for (const r of targets) await deleteAllEntries(db, r);
  await deleteSyncStates(db, targets);
  return { cleared: targets };
}
