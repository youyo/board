/**
 * sync_state(Go 版 `internal/cache/sync_state.go`)。リソースごとの同期状態・ロック・full の進捗。
 * Go から外した列: profile_name / last_daily_refresh_date(daily auto-refresh の廃止)/ cache_version / schema_version。
 * 足した列: full_generation / full_next_page / full_started_at / full_max_updated_at(full の再開)、delta_next_page / delta_started_at / delta_max_updated_at(delta の再開。v3)、query_version / full_query_version / delta_query_version(クエリの版。v4)。
 */

import { LIST_FILTER_SUPPORT } from "../board/openapi-support";
import { ensureSchema } from "./schema";

/**
 * キャッシュを作ったクエリの版。refresh の一覧クエリの内容(どのレコードを取るか)を変えたら上げる。
 * 版 1 = アーカイブ済み・失注を含めて取る(include_archive_flg=1 / include_lost_flg=1。issue #35)。
 * 版が違う(または NULL=旧コード)の同期は『未同期』として扱い、次の refresh は最初から取り直す。
 */
export const CACHE_QUERY_VERSION = 1;

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
  /** 完了した同期のクエリの版(v4)。CACHE_QUERY_VERSION と一致しなければ未同期。NULL は旧コードの同期。 */
  query_version: number | null;
  /** 進行中の full の進捗を保存した版。CACHE_QUERY_VERSION と一致するときだけ再開する。 */
  full_query_version: number | null;
  delta_query_version: number | null;
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
 * 版(query_version など)を要求するリソースか。一覧が include_archive_flg を受けるリソースだけ(クエリが変わったため。issue #35)。
 * 受けないリソース(invoices / payments / users / project_costs)はクエリが変わっていないので、旧コードの同期・進捗もそのまま有効。
 */
export function requiresQueryVersion(resource: string): boolean {
  return LIST_FILTER_SUPPORT[resource]?.includeArchiveFlg === true;
}

/** 保存された版が、現在のクエリで作られたものとみなせるか(版を要求しないリソースは常に true)。 */
export function queryVersionCurrent(resource: string, stored: number | null | undefined): boolean {
  return !requiresQueryVersion(resource) || stored === CACHE_QUERY_VERSION;
}

/**
 * キャッシュを読んでよい状態か(Go の `tryCacheFilter` の `state != nil` に相当)。
 * Go はロックを取っただけでも状態が作られるが、ここでは実際に同期した記録(last_synced_at)があることを条件にする。
 * full が途中(partial)のときは last_synced_at を進めないので、新規のリソースは未同期扱いになる。
 * 版を要求するリソースで query_version が CACHE_QUERY_VERSION と違う(旧コードの同期)ときも未同期扱い(アーカイブ・失注が欠けているため。issue #35)。
 */
export function hasSynced(state: SyncState | null): boolean {
  return (
    state?.last_synced_at != null && queryVersionCurrent(state.resource_name, state.query_version)
  );
}

/** 現在の版で保存した full の進捗があるか(版を要求するリソースの旧コードの進捗は再開せず、表示にも出さない)。 */
export function fullInProgress(state: SyncState | null): boolean {
  return (
    state?.full_next_page != null &&
    queryVersionCurrent(state.resource_name, state.full_query_version)
  );
}

/** 現在の版で保存した delta の進捗があるか。 */
export function deltaInProgress(state: SyncState | null): boolean {
  return (
    state?.delta_next_page != null &&
    queryVersionCurrent(state.resource_name, state.delta_query_version)
  );
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
