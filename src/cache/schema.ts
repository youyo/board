/**
 * D1 キャッシュのスキーマ(Go 版 `internal/cache/schema.go` `migrate.go` を移したもの)。
 *
 * - profile の列は持たない(1 デプロイ = 1 BOARD アカウント)。
 * - Worker が isolate ごとの初回利用時に `ensureSchema` で冪等な DDL を適用する。`wrangler d1 migrations` は使わない。
 * - 版の管理は `cache_meta.db_schema_version`。将来の変更は MIGRATIONS に版を足す(各版は冪等に書く)。
 */

export const SCHEMA_VERSION = 4;

const DDL_V1: string[] = [
  // エンティティ単位の JSON。generation は full refresh の世代(古い世代の行は full の完了時に消す)。
  `CREATE TABLE IF NOT EXISTS resource_cache (
    resource_name TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    payload_json TEXT NOT NULL,
    updated_at TEXT,
    fetched_at TEXT NOT NULL,
    generation INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (resource_name, entity_id)
  )`,
  `CREATE INDEX IF NOT EXISTS idx_resource_cache_updated
    ON resource_cache (resource_name, updated_at)`,
  `CREATE INDEX IF NOT EXISTS idx_resource_cache_generation
    ON resource_cache (resource_name, generation)`,
  // リソースごとの同期状態・ロック・full の進捗。
  `CREATE TABLE IF NOT EXISTS sync_state (
    resource_name TEXT PRIMARY KEY,
    last_synced_at TEXT,
    cursor_updated_at TEXT,
    last_full_synced_at TEXT,
    last_sync_mode TEXT,
    last_sync_status TEXT,
    must_full_resync INTEGER NOT NULL DEFAULT 0,
    expired_at TEXT,
    invalidate_reason TEXT,
    last_error_at TEXT,
    last_error_code TEXT,
    last_error_message TEXT,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    refresh_started_at TEXT,
    refresh_owner TEXT,
    full_generation INTEGER NOT NULL DEFAULT 0,
    full_next_page INTEGER,
    full_started_at TEXT,
    full_max_updated_at TEXT,
    full_prune_skipped INTEGER NOT NULL DEFAULT 0
  )`,
  `CREATE TABLE IF NOT EXISTS cache_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
  )`,
];

// v2(M05): 書類 ID → 案件 ID の逆引き(find 層)。書類の API は案件 ID を返さないので、
// projects を response_group 付きで走査して得た対応を残す。走査はページ単位で再開できる(締め切りで止まっても進捗が残る)。
const DDL_V2: string[] = [
  `CREATE TABLE IF NOT EXISTS document_project_map (
    document_kind TEXT NOT NULL,
    document_id INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    PRIMARY KEY (document_kind, document_id)
  )`,
  `CREATE TABLE IF NOT EXISTS document_scan_state (
    document_kind TEXT PRIMARY KEY,
    next_page INTEGER,
    started_at TEXT,
    completed_at TEXT,
    lock_started_at TEXT,
    lock_owner TEXT
  )`,
];

// SQLite の ADD COLUMN には IF NOT EXISTS がないので、既にある列は無視して冪等にする。
async function addColumns(
  db: D1Database,
  table: string,
  columns: readonly string[],
): Promise<void> {
  for (const col of columns) {
    try {
      await db.prepare(`ALTER TABLE ${table} ADD COLUMN ${col}`).run();
    } catch (e) {
      // 適用済み(または同時に別の isolate が追加した)なら問題ない。
      if (
        !/duplicate column name/i.test(
          String(e instanceof Error ? `${e.message} ${e.cause ?? ""}` : e),
        )
      ) {
        throw e;
      }
    }
  }
}

// v3: delta の再開(full と同じ方式)。進行中の delta の次のページ・開始時刻・見た updated_at の最大値。
const DELTA_RESUME_COLUMNS = [
  "delta_next_page INTEGER",
  "delta_started_at TEXT",
  "delta_max_updated_at TEXT",
];

const applyV3 = (db: D1Database) => addColumns(db, "sync_state", DELTA_RESUME_COLUMNS);

// v4(issue #35): キャッシュを作ったクエリの版。既存の行は NULL(= 旧コードの同期。アーカイブ・失注が欠けている)で、
// 『未同期』として扱う(sync-state の CACHE_QUERY_VERSION)。full_query_version / delta_query_version は、進行中の進捗を保存した版
// (旧コードの進捗=NULL は再開しない)。last_synced_at などを消さないのは、実行中の旧 refresh が後から書き戻して同期済みを復活させうるため。
const QUERY_VERSION_COLUMNS = [
  "query_version INTEGER",
  "full_query_version INTEGER",
  "delta_query_version INTEGER",
];

// document_scan_state の query_version: 走査の進捗・完了を保存した版。旧コード(NULL)の進捗はアーカイブ・失注の案件を飛ばしたページ列なので、
// 再開も完了の信用もしない(対応表 document_project_map は誤りではなく欠けているだけなので消さない)。
const SCAN_VERSION_COLUMNS = ["query_version INTEGER"];

const applyV4 = async (db: D1Database) => {
  await addColumns(db, "sync_state", QUERY_VERSION_COLUMNS);
  await addColumns(db, "document_scan_state", SCAN_VERSION_COLUMNS);
};

const MIGRATIONS: Array<{
  version: number;
  ddl?: string[];
  apply?: (db: D1Database) => Promise<void>;
}> = [
  { version: 1, ddl: DDL_V1 },
  { version: 2, ddl: DDL_V2 },
  { version: 3, apply: applyV3 },
  { version: 4, apply: applyV4 },
];

const VERSION_KEY = "db_schema_version";

/** 現在の版を読む。cache_meta がまだなければ 0。 */
async function currentVersion(db: D1Database): Promise<number> {
  try {
    const row = await db
      .prepare("SELECT value FROM cache_meta WHERE key = ?")
      .bind(VERSION_KEY)
      .first<{ value: string }>();
    const n = row ? Number.parseInt(row.value, 10) : 0;
    return Number.isFinite(n) ? n : 0;
  } catch {
    // テーブルがない(初回)。
    return 0;
  }
}

/** 足りない版の DDL を流して版を記録する。何度呼んでも同じ結果になる。 */
export async function applySchema(db: D1Database): Promise<void> {
  const current = await currentVersion(db);
  if (current >= SCHEMA_VERSION) return;
  for (const m of MIGRATIONS) {
    if (m.version <= current) continue;
    if (m.ddl) await db.batch(m.ddl.map((sql) => db.prepare(sql)));
    if (m.apply) await m.apply(db);
  }
  await db.batch([
    db
      .prepare("INSERT OR REPLACE INTO cache_meta (key, value) VALUES (?, ?)")
      .bind(VERSION_KEY, String(SCHEMA_VERSION)),
  ]);
}

// D1 ごとに 1 回だけ適用する(isolate の寿命の間)。失敗は記憶せず、次の呼び出しでやり直す。
const applied = new WeakMap<D1Database, Promise<void>>();

export function ensureSchema(db: D1Database): Promise<void> {
  let p = applied.get(db);
  if (!p) {
    p = applySchema(db).catch((e) => {
      applied.delete(db);
      throw e;
    });
    applied.set(db, p);
  }
  return p;
}
