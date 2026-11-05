/**
 * sync_state(Go 版 `internal/cache/sync_state.go`)。リソースごとの同期状態・ロック・full の進捗。
 * Go から外した列: profile_name / last_daily_refresh_date(daily auto-refresh の廃止)/ cache_version / schema_version。
 * 足した列: full_generation / full_next_page / full_started_at / full_max_updated_at(full の再開)、delta_next_page / delta_started_at / delta_max_updated_at(delta の再開。v3)。
 */

import { ensureSchema } from "./schema";

export interface SyncState {
  resource_name: string;
  last_synced_at: string | null;
  cursor_updated_at: string | null;
  last_full_synced_at: string | null;
  last_sync_mode: string | null;
  last_sync_status: string | null;
  must_full_resync: number;
  expired_at: string | null;
  invalidate_reason: string | null;
  last_error_at: string | null;
  last_error_code: string | null;
  last_error_message: string | null;
  consecutive_failures: number;
  /** ロックの取得時刻(ISO)。NULL なら誰も保持していない。 */
  refresh_started_at: string | null;
  refresh_owner: string | null;
  /** 最新の世代(進行中、または完了済み)。delta の書き込みもこの世代を付ける。 */
  full_generation: number;
  /** 次に取るページ。NULL なら full は進行していない(未開始、または完了)。 */
  full_next_page: number | null;
  full_started_at: string | null;
  /** 進行中の full で見た updated_at の最大値(完了時の delta カーソルの元)。 */
  full_max_updated_at: string | null;
  /** 直近の full の完了時に、件数の照合に通らず旧世代を消さなかった(1)。次の full で取り直す。 */
  full_prune_skipped: number;
  /** 進行中の delta の次に取るページ。NULL なら delta は進行していない。カーソルは完了するまで動かさない。 */
  delta_next_page: number | null;
  delta_started_at: string | null;
  /** 進行中の delta で見た updated_at の最大値(完了時のカーソルの元)。 */
  delta_max_updated_at: string | null;
}

export async function getSyncState(db: D1Database, resource: string): Promise<SyncState | null> {
  await ensureSchema(db);
  return db
    .prepare("SELECT * FROM sync_state WHERE resource_name = ?")
    .bind(resource)
    .first<SyncState>();
}

export async function listSyncStates(db: D1Database): Promise<SyncState[]> {
  await ensureSchema(db);
  const r = await db.prepare("SELECT * FROM sync_state ORDER BY resource_name").all<SyncState>();
  return r.results;
}

/**
 * キャッシュを読んでよい状態か(Go の `tryCacheFilter` の `state != nil` に相当)。
 * Go はロックを取っただけでも状態が作られるが、ここでは実際に同期した記録(last_synced_at)があることを条件にする。
 * full が途中(partial)のときは last_synced_at を進めないので、新規のリソースは未同期扱いになる。
 */
export function hasSynced(state: SyncState | null): boolean {
  return state?.last_synced_at != null;
}

/** 行がなければ作る(UPDATE の前提)。 */
export function ensureRowStatement(db: D1Database, resource: string): D1PreparedStatement {
  return db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES (?)").bind(resource);
}

/** 失敗の記録。message はコードだけ(未知の例外の文面は保存しない)。 */
export async function markError(
  db: D1Database,
  resource: string,
  code: string,
  atIso: string,
  /** 指定すると、ロックの所有者がこの値のときだけ記録する(ロックを失った保持者が他人の状態を汚さない)。 */
  owner?: string,
): Promise<void> {
  await ensureSchema(db);
  await db.batch([
    ensureRowStatement(db, resource),
    db
      .prepare(
        `UPDATE sync_state SET last_sync_status = 'error', last_error_at = ?, last_error_code = ?,
         last_error_message = ?, consecutive_failures = consecutive_failures + 1
         WHERE resource_name = ? AND (? IS NULL OR refresh_owner = ?)`,
      )
      .bind(atIso, code, code, resource, owner ?? null, owner ?? null),
  ]);
}

/** 手動で期限切れの印を付ける(Go の `Expire`)。読み取りの挙動は変えない(情報用)。 */
export async function expireStates(
  db: D1Database,
  resources: readonly string[],
  atIso: string,
): Promise<void> {
  await ensureSchema(db);
  await db.batch(
    resources.flatMap((r) => [
      ensureRowStatement(db, r),
      db
        .prepare(
          "UPDATE sync_state SET expired_at = ?, invalidate_reason = 'manual' WHERE resource_name = ?",
        )
        .bind(atIso, r),
    ]),
  );
}

export async function deleteSyncStates(
  db: D1Database,
  resources: readonly string[],
): Promise<void> {
  await ensureSchema(db);
  if (resources.length === 0) return;
  await db.batch(
    resources.map((r) => db.prepare("DELETE FROM sync_state WHERE resource_name = ?").bind(r)),
  );
}
