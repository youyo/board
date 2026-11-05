/**
 * resource_cache(Go 版 `internal/cache/resource_cache.go`)。エンティティ単位の JSON を持つ。
 * D1 は 1 文あたりのバインド数が 100 までなので、1 件ずつの upsert 文を `batch` で原子的に流す。
 */

import { BoardError } from "../errors";
import { LOCK_STALE_MS } from "../refresh/lock";
import { ensureSchema } from "./schema";
import { type Clock, isoSeconds, systemClock } from "./time";

export interface CacheEntry<T = unknown> {
  entity: T;
  entity_id: number;
  updated_at: string | null;
  fetched_at: string;
  generation: number;
}

export interface UpsertOptions {
  /** この行に付ける世代。full の進行中はその世代、それ以外は sync_state.full_generation。既定 0。 */
  generation?: number;
  now?: number;
  /** 1 件だけ upsert するときの保存キー(エンティティの id ではなく、取得に使った id で引けるようにする。document 用)。 */
  entityId?: number;
  /** 指定すると、sync_state の `refresh_owner` がこの値のときだけ書く(同じ batch の中で判定する)。ロックを失った保持者の書き込みを無効にする。 */
  guardOwner?: string;
}

const SQL_UPSERT = `INSERT OR REPLACE INTO resource_cache
  (resource_name, entity_id, payload_json, updated_at, fetched_at, generation)
  VALUES (?, ?, ?, ?, ?, ?)`;

const OWNER_EXISTS = `EXISTS (SELECT 1 FROM sync_state WHERE resource_name = ? AND refresh_owner = ?)`;

/** 所有者の条件つき upsert。ロックを失っていれば 0 行の SELECT になり、何も書かれない。 */
const SQL_UPSERT_GUARDED = `INSERT OR REPLACE INTO resource_cache
  (resource_name, entity_id, payload_json, updated_at, fetched_at, generation)
  SELECT ?, ?, ?, ?, ?, ? WHERE ${OWNER_EXISTS}`;

function entityId(entity: unknown): number {
  const id = (entity as { id?: unknown } | null)?.id;
  if (typeof id !== "number" || !Number.isSafeInteger(id) || id < 1) {
    throw new BoardError("api_error", "BOARD API returned an entity without a valid id.", false);
  }
  return id;
}

function updatedAtOf(entity: unknown): string | null {
  const v = (entity as { updated_at?: unknown } | null)?.updated_at;
  return typeof v === "string" && v !== "" ? v : null;
}

/** upsert 文の組み立て(refresh が sync_state の更新と同じ `batch` に入れるため公開)。 */
export function upsertStatements(
  db: D1Database,
  resource: string,
  entities: readonly unknown[],
  opts: UpsertOptions = {},
): D1PreparedStatement[] {
  const fetchedAt = isoSeconds(opts.now ?? systemClock());
  const generation = opts.generation ?? 0;
  const guard = opts.guardOwner;
  return entities.map((e) => {
    const params: unknown[] = [
      resource,
      entities.length === 1 && opts.entityId !== undefined ? opts.entityId : entityId(e),
      JSON.stringify(e),
      updatedAtOf(e),
      fetchedAt,
      generation,
    ];
    if (guard === undefined) return db.prepare(SQL_UPSERT).bind(...params);
    return db.prepare(SQL_UPSERT_GUARDED).bind(...params, resource, guard);
  });
}

/** 複数件を 1 つのバッチ(原子的)で upsert する。入れた件数を返す。 */
export async function upsertEntities(
  db: D1Database,
  resource: string,
  entities: readonly unknown[],
  opts: UpsertOptions = {},
): Promise<number> {
  if (entities.length === 0) return 0;
  await ensureSchema(db);
  await db.batch(upsertStatements(db, resource, entities, opts));
  return entities.length;
}

/**
 * 単体取得(キャッシュミスで API から 1 件取った結果)の保存。1 文の条件付き INSERT で、読んでから書くまでの隙間を作らない。
 * - 世代: INSERT の中で `sync_state.full_generation` を読む(行がなければ 0)。`cache_clear` で世代が数え直しになっても、
 *   古い大きい世代の行が戻って full の削除や件数の照合を狂わせることがない。
 * - ロック: その resource のロックが取られている間は書かない。『取られている』は `tryAcquireLock` の定義と同じ
 *   (`refresh_started_at` があり、`now - LOCK_STALE_MS` 以降。stale になったものは奪える = 保持していない扱い)。
 * - ID: 応答の `id` が 1 以上の安全な整数で、要求した ID と同じときだけ書く(それ以外は何もしない)。
 * - 新しさ: API を待っている間に別の refresh / get が同じ ID の行をもっと新しく書いていたら、その行を古い内容で上書きしない。
 *   既存の行がない、または既存の `fetched_at < isoSeconds(fetchStartedAt)`(取得を始めた時刻より前に書かれた行)のときだけ書く。
 *   `INSERT OR IGNORE` にしないのは、`refresh=full` の書類 1 件の取り直しで古い行を更新できる必要があるため。
 *   `fetched_at` は秒精度なので、同じ秒は『新しい行を守る』側に倒す(上書きしない)。取り違えた場合の被害は、
 *   同じ秒に書かれた古い行が次の取得まで残るだけで、古い内容が新しい内容を消すより軽い。
 * 書いたかどうかを返す。
 */
export async function cacheFetchedEntity(
  db: D1Database,
  resource: string,
  entity: unknown,
  opts: {
    requestedId: number /** API を呼ぶ直前の時刻(epoch ms)。 */;
    fetchStartedAt: number;
    now?: number;
  },
): Promise<boolean> {
  const id = (entity as { id?: unknown } | null)?.id;
  if (typeof id !== "number" || !Number.isSafeInteger(id) || id < 1 || id !== opts.requestedId) {
    return false;
  }
  await ensureSchema(db);
  const now = opts.now ?? systemClock();
  const r = await db
    .prepare(
      `INSERT INTO resource_cache
        (resource_name, entity_id, payload_json, updated_at, fetched_at, generation)
       SELECT ?, ?, ?, ?, ?, COALESCE((SELECT full_generation FROM sync_state WHERE resource_name = ?), 0)
       WHERE NOT EXISTS (
         SELECT 1 FROM sync_state WHERE resource_name = ? AND refresh_started_at IS NOT NULL AND refresh_started_at >= ?
       )
       ON CONFLICT (resource_name, entity_id) DO UPDATE SET
         payload_json = excluded.payload_json,
         updated_at = excluded.updated_at,
         fetched_at = excluded.fetched_at,
         generation = excluded.generation
       WHERE resource_cache.fetched_at < ?`,
    )
    .bind(
      resource,
      id,
      JSON.stringify(entity),
      updatedAtOf(entity),
      isoSeconds(now),
      resource,
      resource,
      isoSeconds(now - LOCK_STALE_MS),
      isoSeconds(opts.fetchStartedAt),
    )
    .run();
  return r.meta.changes === 1;
}

interface Row {
  entity_id: number;
  payload_json: string;
  updated_at: string | null;
  fetched_at: string;
  generation: number;
}

function toEntry<T>(r: Row): CacheEntry<T> {
  return {
    entity: JSON.parse(r.payload_json) as T,
    entity_id: r.entity_id,
    updated_at: r.updated_at,
    fetched_at: r.fetched_at,
    generation: r.generation,
  };
}

export async function getEntry<T = unknown>(
  db: D1Database,
  resource: string,
  id: number,
): Promise<CacheEntry<T> | null> {
  await ensureSchema(db);
  const r = await db
    .prepare(
      "SELECT entity_id, payload_json, updated_at, fetched_at, generation FROM resource_cache WHERE resource_name = ? AND entity_id = ?",
    )
    .bind(resource, id)
    .first<Row>();
  return r ? toEntry<T>(r) : null;
}

/**
 * updated_at の昇順(NULL が先頭。Go と同じ)。同じなら id 順にして順序を決定的にする。
 * `limit` が 1 以上なら SQL の LIMIT で先頭からその件数だけ読む(それ以外の行は読まず、JSON も解析しない)。0・未指定は全件。
 */
export async function listEntries<T = unknown>(
  db: D1Database,
  resource: string,
  opts: { limit?: number } = {},
): Promise<Array<CacheEntry<T>>> {
  await ensureSchema(db);
  const limit = opts.limit !== undefined && opts.limit > 0 ? Math.floor(opts.limit) : undefined;
  const r = await db
    .prepare(
      `SELECT entity_id, payload_json, updated_at, fetched_at, generation FROM resource_cache WHERE resource_name = ? ORDER BY updated_at ASC, entity_id ASC${limit === undefined ? "" : " LIMIT ?"}`,
    )
    .bind(...(limit === undefined ? [resource] : [resource, limit]))
    .all<Row>();
  return r.results.map((row) => toEntry<T>(row));
}

export async function countEntries(db: D1Database, resource: string): Promise<number> {
  await ensureSchema(db);
  const r = await db
    .prepare("SELECT count(*) AS c FROM resource_cache WHERE resource_name = ?")
    .bind(resource)
    .first<{ c: number }>();
  return r?.c ?? 0;
}

export async function deleteEntry(db: D1Database, resource: string, id: number): Promise<void> {
  await ensureSchema(db);
  await db
    .prepare("DELETE FROM resource_cache WHERE resource_name = ? AND entity_id = ?")
    .bind(resource, id)
    .run();
}

export async function deleteAllEntries(db: D1Database, resource: string): Promise<void> {
  await ensureSchema(db);
  await db.prepare("DELETE FROM resource_cache WHERE resource_name = ?").bind(resource).run();
}

export interface PruneOptions {
  /** 指定すると、所有者がこの値のときだけ消す。 */
  guardOwner?: string;
  /** BOARD の総件数。新しい世代以降の行数がこれに満たなければ消さない(同じ batch の中、upsert のあとで判定)。 */
  expectedTotal?: number | null;
}

/**
 * 件数の照合を通ったか(新しい世代以降の行数 >= 総件数)を表す SQL の条件。`?` は (total, resource, generation, total) の順。
 * total が NULL(ヘッダなし)なら常に真。
 */
export const PRUNE_OK_SQL = `(? IS NULL OR (SELECT count(*) FROM resource_cache WHERE resource_name = ? AND generation >= ?) >= ?)`;

/** 指定の世代より古い行の削除文(full の完了時に、最後のページの upsert と同じバッチに入れる)。 */
export function deleteOlderGenerationsStatement(
  db: D1Database,
  resource: string,
  generation: number,
  opts: PruneOptions = {},
): D1PreparedStatement {
  const total = opts.expectedTotal ?? null;
  let sql = `DELETE FROM resource_cache WHERE resource_name = ? AND generation < ? AND ${PRUNE_OK_SQL}`;
  const params: unknown[] = [resource, generation, total, resource, generation, total];
  if (opts.guardOwner !== undefined) {
    sql += ` AND ${OWNER_EXISTS}`;
    params.push(resource, opts.guardOwner);
  }
  return db.prepare(sql).bind(...params);
}

export async function deleteOlderGenerations(
  db: D1Database,
  resource: string,
  generation: number,
): Promise<void> {
  await ensureSchema(db);
  await deleteOlderGenerationsStatement(db, resource, generation).run();
}

export type { Clock };
