/**
 * キャッシュの運用操作(Go 版 `internal/cli/cache.go` の status / expire / clear)。M06 の cache 系ツールの土台。
 * expire は Go と同じく sync_state に印を付けるだけで、読み取りの挙動は変えない(daily auto-refresh の廃止後は情報用)。
 */

import { RESOURCES } from "../board/resources";
import { ArgumentError, RefreshInProgressError } from "../errors";
import { LOCK_STALE_MS, type Lock, releaseLock, tryAcquireLock } from "../refresh/lock";
import { SCAN_LOCK_STALE_MS } from "../repository/document-index";
import { DOCUMENT_KINDS } from "../repository/document-kinds";
import { getScanState, releaseScanLock, tryAcquireScanLock } from "./document-map";
import { ensureSchema } from "./schema";
import { expireStates, listSyncStates, type SyncState } from "./sync-state";
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
  /** 直近の full の完了時に、件数の照合に通らず旧世代を消さなかった。次の full で取り直す。 */
  full_prune_skipped: boolean;
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
      full_prune_skipped: (s?.full_prune_skipped ?? 0) !== 0,
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

/**
 * 行と同期状態を消す。先にロックを取る(refresh と同じ排他)ので、refresh 中のリソースがあれば何も消さずに RefreshInProgressError。
 * 確認から削除までの間に refresh が始まることはない(ロックを持っているので TryLock が弾かれる)。
 * 削除は所有者を条件に 1 つの batch で行い、ロックを奪われていた場合は何も消さず、奪った側のロック行も消さない。
 */
export async function clearCache(
  db: D1Database,
  opts: AdminOptions & {
    resources?: readonly string[];
    /** テスト用: ロックを取り終えたあと・削除の前に呼ばれる。 */
    hooks?: { afterLock?: () => Promise<void> };
  } = {},
): Promise<{ cleared: readonly string[] }> {
  const targets = validate(opts.resources);
  await ensureSchema(db);
  const locks: Lock[] = [];
  try {
    for (const r of targets) locks.push(await tryAcquireLock(db, r, { now: opts.now }));
    await opts.hooks?.afterLock?.();
    const results = await db.batch(
      locks.flatMap((l) => [
        db
          .prepare(
            `DELETE FROM resource_cache WHERE resource_name = ?
             AND EXISTS (SELECT 1 FROM sync_state WHERE resource_name = ? AND refresh_owner = ?)`,
          )
          .bind(l.resource, l.resource, l.owner),
        db
          .prepare("DELETE FROM sync_state WHERE resource_name = ? AND refresh_owner = ?")
          .bind(l.resource, l.owner),
      ]),
    );
    // sync_state の削除が空振り = ロックを奪われていた(その分の行も消えていない)。
    const lost = locks.findIndex((_, i) => results[i * 2 + 1]?.meta.changes !== 1);
    if (lost >= 0) throw new RefreshInProgressError(locks[lost]?.resource ?? "");
  } finally {
    // 削除で消えた行には何も起きない。失敗・奪われた場合は、自分の分だけ解放する。
    await Promise.all(locks.map((l) => releaseLock(db, l.resource, l.owner)));
  }
  return { cleared: targets };
}

// --- cache_status / cache_clear(M06) --------------------------------------------------------

export type ReportRefreshState = "complete" | "partial" | "failed" | "not_synced";

export interface CacheReportResource {
  resource: string;
  /** 最終同期(delta または full の完了)。未取得なら無い。 */
  cached_at?: string;
  /** 最終 full refresh の完了。未取得なら無い。 */
  full_refreshed_at?: string;
  /** complete / partial(full が途中)/ failed(直近の refresh が失敗)/ not_synced(一度も同期していない)。 */
  refresh_state: ReportRefreshState;
  row_count: number;
  full_prune_skipped: boolean;
  /** refresh 中(stale でないロックがある)。 */
  locked: boolean;
  last_error_code?: string;
}

export interface DocumentScanReport {
  document_kind: string;
  /** not_started / in_progress(next_page から再開する)/ complete。 */
  scan_state: "not_started" | "in_progress" | "complete";
  mapped_rows: number;
  next_page?: number;
  started_at?: string;
  completed_at?: string;
  locked: boolean;
}

function refreshStateOf(s: SyncState | undefined): ReportRefreshState {
  if (s?.full_next_page != null) return "partial";
  if (s?.last_sync_status === "error") return "failed";
  return s?.last_synced_at ? "complete" : "not_synced";
}

/** cache_status の本体。読み取り専用(ロックも取らない)。 */
export async function cacheReport(
  db: D1Database,
  opts: AdminOptions = {},
): Promise<{ resources: CacheReportResource[]; document_scans: DocumentScanReport[] }> {
  const now = (opts.now ?? systemClock)();
  const rows = await cacheStatus(db, opts);
  const states = new Map((await listSyncStates(db)).map((s) => [s.resource_name, s]));
  const resources = rows.map((r): CacheReportResource => {
    const s = states.get(r.resource);
    const out: CacheReportResource = {
      resource: r.resource,
      refresh_state: refreshStateOf(s),
      row_count: r.cached_rows,
      full_prune_skipped: r.full_prune_skipped,
      locked: r.refreshing,
    };
    if (r.last_synced_at) out.cached_at = r.last_synced_at;
    if (r.last_full_synced_at) out.full_refreshed_at = r.last_full_synced_at;
    if (out.refresh_state === "failed" && r.last_error_code)
      out.last_error_code = r.last_error_code;
    return out;
  });

  const counts = await db
    .prepare("SELECT document_kind, count(*) AS c FROM document_project_map GROUP BY document_kind")
    .all<{ document_kind: string; c: number }>();
  const mapped = new Map(counts.results.map((r) => [r.document_kind, r.c]));
  const document_scans: DocumentScanReport[] = [];
  for (const kind of DOCUMENT_KINDS) {
    const scan = await getScanState(db, kind);
    const lockedAt = scan?.lock_started_at ? Date.parse(scan.lock_started_at) : Number.NaN;
    const report: DocumentScanReport = {
      document_kind: kind,
      scan_state: scan?.completed_at
        ? "complete"
        : scan?.next_page != null
          ? "in_progress"
          : "not_started",
      mapped_rows: mapped.get(kind) ?? 0,
      locked: !Number.isNaN(lockedAt) && lockedAt >= now - SCAN_LOCK_STALE_MS,
    };
    if (scan?.next_page != null) report.next_page = scan.next_page;
    if (scan?.started_at) report.started_at = scan.started_at;
    if (scan?.completed_at) report.completed_at = scan.completed_at;
    document_scans.push(report);
  }
  return { resources, document_scans };
}

/**
 * 書類 ID → 案件 ID の逆引き表と走査状態を全種別について消す(対応は不変なので、通常の clear では消さない)。
 * clearCache と同じく、先に種別ごとの走査ロックを取り、所有者を条件にした 1 つの batch で消す。
 * 走査中の種別があれば何も消さずに RefreshInProgressError。
 */
export async function clearDocumentMap(
  db: D1Database,
  opts: AdminOptions & { hooks?: { afterLock?: () => Promise<void> } } = {},
): Promise<void> {
  await ensureSchema(db);
  const nowMs = (opts.now ?? systemClock)();
  const owner = crypto.randomUUID();
  const held: string[] = [];
  try {
    for (const kind of DOCUMENT_KINDS) {
      if (!(await tryAcquireScanLock(db, kind, owner, nowMs, SCAN_LOCK_STALE_MS))) {
        throw new RefreshInProgressError(`document_map:${kind}`);
      }
      held.push(kind);
    }
    await opts.hooks?.afterLock?.();
    const results = await db.batch(
      held.flatMap((kind) => [
        db
          .prepare(
            `DELETE FROM document_project_map WHERE document_kind = ?
             AND EXISTS (SELECT 1 FROM document_scan_state WHERE document_kind = ? AND lock_owner = ?)`,
          )
          .bind(kind, kind, owner),
        db
          .prepare("DELETE FROM document_scan_state WHERE document_kind = ? AND lock_owner = ?")
          .bind(kind, owner),
      ]),
    );
    // scan_state の削除が空振り = ロックを奪われていた(その種別の対応表も消えていない)。
    const lost = held.findIndex((_, i) => results[i * 2 + 1]?.meta.changes !== 1);
    if (lost >= 0) throw new RefreshInProgressError(`document_map:${held[lost]}`);
  } finally {
    // 消えた行には何も起きない。失敗・奪われた場合は、自分の分だけ解放する。
    await Promise.all(held.map((kind) => releaseScanLock(db, kind, owner)));
  }
}
