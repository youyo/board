/**
 * issue #35: 版(query_version)が違う同期は、鮮度表示(cacheInfoFromState)と cache_status で『未同期』として出す。
 * LLM が cached_at の有無で『未取得』と判別し、refresh=full を指示できるようにする。
 */
import { beforeEach, describe, expect, it } from "vitest";
import { cacheReport } from "../../src/cache/admin";
import { cacheInfoFromState } from "../../src/cache/info";
import { applySchema, SCHEMA_VERSION } from "../../src/cache/schema";
import { CACHE_QUERY_VERSION, getSyncState } from "../../src/cache/sync-state";
import { fakeClock, resetCache, testDb } from "../helpers/d1";

const clock = fakeClock();

beforeEach(async () => {
  await resetCache();
});

async function seed(db: D1Database, version: number | null, extra = "") {
  await db.batch([
    db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES ('projects')"),
    db
      .prepare(
        `UPDATE sync_state SET last_synced_at='2026-11-04T00:00:00Z', last_full_synced_at='2026-11-01T00:00:00Z',
         last_sync_status='success', query_version = ? ${extra} WHERE resource_name='projects'`,
      )
      .bind(version),
  ]);
}

describe("cacheInfoFromState: 版違いは cached_at / full_refreshed_at を出さない", () => {
  it("版が一致: 従来どおり出る", async () => {
    const db = testDb();
    await seed(db, CACHE_QUERY_VERSION);
    expect(cacheInfoFromState("projects", await getSyncState(db, "projects"))).toEqual({
      resource: "projects",
      cached_at: "2026-11-04T00:00:00Z",
      full_refreshed_at: "2026-11-01T00:00:00Z",
    });
  });

  it("旧コードの行(query_version が NULL): 出ない(未取得と区別できる)", async () => {
    const db = testDb();
    await seed(db, null);
    expect(cacheInfoFromState("projects", await getSyncState(db, "projects"))).toEqual({
      resource: "projects",
    });
  });

  it("別の版: 出ない", async () => {
    const db = testDb();
    await seed(db, CACHE_QUERY_VERSION + 1);
    const info = cacheInfoFromState("projects", await getSyncState(db, "projects"));
    expect(info.cached_at).toBeUndefined();
    expect(info.full_refreshed_at).toBeUndefined();
  });

  it("旧版の full の進捗は partial と表示しない(再開しないため)。同じ版の進捗は partial", async () => {
    const db = testDb();
    await seed(db, null, ", full_next_page = 3");
    expect(cacheInfoFromState("projects", await getSyncState(db, "projects")).refresh_state).toBe(
      undefined,
    );
    await db
      .prepare("UPDATE sync_state SET full_query_version = ? WHERE resource_name='projects'")
      .bind(CACHE_QUERY_VERSION)
      .run();
    expect(cacheInfoFromState("projects", await getSyncState(db, "projects")).refresh_state).toBe(
      "partial",
    );
  });
});

describe("cache_status(cacheReport): 版違いは not_synced", () => {
  const find = async (db: D1Database) =>
    (await cacheReport(db, { now: clock.now })).resources.find((r) => r.resource === "projects");

  it("版が一致: complete で cached_at が出る", async () => {
    const db = testDb();
    await seed(db, CACHE_QUERY_VERSION);
    expect(await find(db)).toMatchObject({
      refresh_state: "complete",
      cached_at: "2026-11-04T00:00:00Z",
      full_refreshed_at: "2026-11-01T00:00:00Z",
    });
  });

  it("旧コードの行: not_synced で cached_at / full_refreshed_at が出ない。row_count は行数のまま", async () => {
    const db = testDb();
    await seed(db, null);
    await db
      .prepare(
        "INSERT INTO resource_cache (resource_name, entity_id, payload_json, fetched_at) VALUES ('projects', 1, '{}', 'x')",
      )
      .run();
    const r = await find(db);
    expect(r).toMatchObject({ refresh_state: "not_synced", row_count: 1 });
    expect(r?.cached_at).toBeUndefined();
    expect(r?.full_refreshed_at).toBeUndefined();
  });

  it("旧版の進捗(full_query_version なし)は partial ではなく not_synced。同じ版の進捗は partial", async () => {
    const db = testDb();
    await seed(db, null, ", full_next_page = 3");
    expect((await find(db))?.refresh_state).toBe("not_synced");
    await db
      .prepare("UPDATE sync_state SET full_query_version = ? WHERE resource_name='projects'")
      .bind(CACHE_QUERY_VERSION)
      .run();
    expect((await find(db))?.refresh_state).toBe("partial");
  });
});

describe("移行 v4: sync_state に版の列を足す", () => {
  const cols = async (db: D1Database) =>
    (await db.prepare("PRAGMA table_info(sync_state)").all<{ name: string }>()).results.map(
      (c) => c.name,
    );

  it("SCHEMA_VERSION は 4", () => {
    expect(SCHEMA_VERSION).toBe(4);
  });

  it("v3 の DB から上がる。既存の行は壊れず query_version は NULL。何度流しても同じ", async () => {
    const db = testDb();
    await applySchema(db);
    await db.batch([
      db.prepare("ALTER TABLE sync_state DROP COLUMN query_version"),
      db.prepare("ALTER TABLE sync_state DROP COLUMN full_query_version"),
      db.prepare("ALTER TABLE sync_state DROP COLUMN delta_query_version"),
      db.prepare(
        "INSERT OR REPLACE INTO sync_state (resource_name, last_synced_at, full_generation) VALUES ('clients', '2026-01-01T00:00:00Z', 4)",
      ),
      db.prepare("UPDATE cache_meta SET value = '3' WHERE key = 'db_schema_version'"),
    ]);
    expect(await cols(db)).not.toContain("query_version");

    await applySchema(db);
    expect(await cols(db)).toEqual(
      expect.arrayContaining(["query_version", "full_query_version", "delta_query_version"]),
    );
    const row = await db
      .prepare("SELECT * FROM sync_state WHERE resource_name = 'clients'")
      .first<Record<string, unknown>>();
    expect(row).toMatchObject({
      last_synced_at: "2026-01-01T00:00:00Z",
      full_generation: 4,
      query_version: null,
      full_query_version: null,
      delta_query_version: null,
    });

    // 版の記録を戻して 2 回目を流しても(列が既にあっても)失敗しない。
    await db.prepare("UPDATE cache_meta SET value = '3' WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    await applySchema(db);
    expect((await cols(db)).filter((c) => c === "query_version")).toHaveLength(1);
    const v = await db
      .prepare("SELECT value FROM cache_meta WHERE key = 'db_schema_version'")
      .first<{ value: string }>();
    expect(v?.value).toBe("4");
  });
});
