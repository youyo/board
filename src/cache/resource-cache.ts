/**
 * resource_cache(Go 版 `internal/cache/resource_cache.go`)。エンティティ単位の JSON を持つ。
 * D1 は 1 文あたりのバインド数が 100 までなので、1 件ずつの upsert 文を `batch` で原子的に流す。
 */

import { BoardError } from "../errors";
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
}

const SQL_UPSERT = `INSERT OR REPLACE INTO resource_cache
  (resource_name, entity_id, payload_json, updated_at, fetched_at, generation)
  VALUES (?, ?, ?, ?, ?, ?)`;

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
  return entities.map((e) =>
    db
      .prepare(SQL_UPSERT)
      .bind(
        resource,
        entities.length === 1 && opts.entityId !== undefined ? opts.entityId : entityId(e),
        JSON.stringify(e),
        updatedAtOf(e),
        fetchedAt,
        generation,
      ),
  );
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

/** updated_at の昇順(NULL が先頭。Go と同じ)。同じなら id 順にして順序を決定的にする。 */
export async function listEntries<T = unknown>(
  db: D1Database,
  resource: string,
): Promise<Array<CacheEntry<T>>> {
  await ensureSchema(db);
  const r = await db
    .prepare(
      "SELECT entity_id, payload_json, updated_at, fetched_at, generation FROM resource_cache WHERE resource_name = ? ORDER BY updated_at ASC, entity_id ASC",
    )
    .bind(resource)
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

/** 指定の世代より古い行の削除文(full の完了時に、最後のページの upsert と同じバッチに入れる)。 */
export function deleteOlderGenerationsStatement(
  db: D1Database,
  resource: string,
  generation: number,
): D1PreparedStatement {
  return db
    .prepare("DELETE FROM resource_cache WHERE resource_name = ? AND generation < ?")
    .bind(resource, generation);
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
