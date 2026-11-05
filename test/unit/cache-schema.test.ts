import { beforeEach, describe, expect, it } from "vitest";
import { applySchema, ensureSchema, SCHEMA_VERSION } from "../../src/cache/schema";
import { resetCache, testDb } from "../helpers/d1";

async function names(db: D1Database, type: "table" | "index"): Promise<string[]> {
  const r = await db
    .prepare(
      "SELECT name FROM sqlite_master WHERE type = ? AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_cf_%' ORDER BY name",
    )
    .bind(type)
    .all<{ name: string }>();
  return r.results.map((x) => x.name);
}

describe("スキーマの適用", () => {
  beforeEach(async () => {
    await resetCache();
  });

  it("テーブルと索引を作り、スキーマ版を cache_meta に書く(v2 で書類 → 案件の逆引きの 2 表を追加)", async () => {
    const db = testDb();
    await applySchema(db);
    expect(await names(db, "table")).toEqual(
      expect.arrayContaining([
        "cache_meta",
        "resource_cache",
        "sync_state",
        "document_project_map",
        "document_scan_state",
      ]),
    );
    expect(SCHEMA_VERSION).toBe(3);
    expect(await names(db, "index")).toEqual(
      expect.arrayContaining(["idx_resource_cache_generation", "idx_resource_cache_updated"]),
    );
    const v = await db
      .prepare("SELECT value FROM cache_meta WHERE key = 'db_schema_version'")
      .first<{ value: string }>();
    expect(v?.value).toBe(String(SCHEMA_VERSION));
  });

  it("何度適用しても同じ結果になり、既存のデータを壊さない(冪等)", async () => {
    const db = testDb();
    await applySchema(db);
    await db
      .prepare(
        "INSERT INTO resource_cache (resource_name, entity_id, payload_json, fetched_at) VALUES ('clients', 1, '{}', 'x')",
      )
      .run();
    const before = await names(db, "table");
    // 版の記録を消して、DDL が実際にもう一度流れる状態で確かめる。
    await db.prepare("DELETE FROM cache_meta WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    await db.prepare("DELETE FROM cache_meta WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    expect(await names(db, "table")).toEqual(before);
    const n = await db.prepare("SELECT count(*) AS c FROM resource_cache").first<{ c: number }>();
    expect(n?.c).toBe(1);
  });

  it("v1 のデータベースに v2 だけを足す(既存のデータを壊さない)", async () => {
    const db = testDb();
    await applySchema(db);
    await db.prepare("DROP TABLE document_project_map").run();
    await db.prepare("DROP TABLE document_scan_state").run();
    await db
      .prepare(
        "INSERT OR REPLACE INTO resource_cache (resource_name, entity_id, payload_json, fetched_at) VALUES ('clients', 9, '{}', 'x')",
      )
      .run();
    await db.prepare("UPDATE cache_meta SET value = '1' WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    expect(await names(db, "table")).toEqual(
      expect.arrayContaining(["document_project_map", "document_scan_state"]),
    );
    const n = await db
      .prepare("SELECT count(*) AS c FROM resource_cache WHERE entity_id = 9")
      .first<{ c: number }>();
    expect(n?.c).toBe(1);
    const v = await db
      .prepare("SELECT value FROM cache_meta WHERE key = 'db_schema_version'")
      .first<{ value: string }>();
    expect(v?.value).toBe(String(SCHEMA_VERSION));
  });

  it("v2 のデータベースに v3(delta の再開用の列)だけを足す。既存の行と同期状態は壊さず、何度流しても同じ", async () => {
    const db = testDb();
    await applySchema(db);
    await db.batch([
      db.prepare("ALTER TABLE sync_state DROP COLUMN delta_next_page"),
      db.prepare("ALTER TABLE sync_state DROP COLUMN delta_started_at"),
      db.prepare("ALTER TABLE sync_state DROP COLUMN delta_max_updated_at"),
      db.prepare(
        "INSERT OR REPLACE INTO sync_state (resource_name, cursor_updated_at, full_generation) VALUES ('clients', '2026-01-01T00:00:00Z', 4)",
      ),
      db.prepare("UPDATE cache_meta SET value = '2' WHERE key = 'db_schema_version'"),
    ]);
    const cols = async () =>
      (await db.prepare("PRAGMA table_info(sync_state)").all<{ name: string }>()).results.map(
        (c) => c.name,
      );
    expect(await cols()).not.toContain("delta_next_page");

    await applySchema(db);
    expect(await cols()).toEqual(
      expect.arrayContaining(["delta_next_page", "delta_started_at", "delta_max_updated_at"]),
    );
    const row = await db
      .prepare("SELECT * FROM sync_state WHERE resource_name = 'clients'")
      .first<Record<string, unknown>>();
    expect(row).toMatchObject({
      cursor_updated_at: "2026-01-01T00:00:00Z",
      full_generation: 4,
      delta_next_page: null,
      delta_started_at: null,
      delta_max_updated_at: null,
    });

    // 版の記録を戻して DDL をもう一度流しても(列が既にあっても)失敗しない。
    await db.prepare("UPDATE cache_meta SET value = '2' WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    expect((await cols()).filter((c) => c === "delta_next_page")).toHaveLength(1);
  });

  it("profile の列を持たない(1 デプロイ = 1 アカウント)", async () => {
    const db = testDb();
    for (const table of ["resource_cache", "sync_state"]) {
      const cols = await db.prepare(`PRAGMA table_info(${table})`).all<{ name: string }>();
      expect(cols.results.map((c) => c.name)).not.toContain("profile_name");
    }
  });

  it("sync_state に full の再開用の列(世代・次のページ)がある", async () => {
    const cols = await testDb().prepare("PRAGMA table_info(sync_state)").all<{ name: string }>();
    expect(cols.results.map((c) => c.name)).toEqual(
      expect.arrayContaining([
        "full_generation",
        "full_next_page",
        "refresh_started_at",
        "refresh_owner",
      ]),
    );
  });
});

describe("ensureSchema(isolate ごとの初回利用時に適用)", () => {
  // 版の記録を消して、未適用の D1 として扱う(テーブルは残るので DDL は冪等に流れる)。
  beforeEach(async () => {
    await testDb().prepare("DELETE FROM cache_meta WHERE key = 'db_schema_version'").run();
  });

  function counting(real: D1Database) {
    const calls = { batch: 0 };
    const db = {
      prepare: (q: string) => real.prepare(q),
      batch: (s: D1PreparedStatement[]) => {
        calls.batch++;
        return real.batch(s);
      },
    } as unknown as D1Database;
    return { db, calls };
  }

  it("同じ D1 に対して 2 回目以降は DDL を流さない", async () => {
    const { db, calls } = counting(testDb());
    await ensureSchema(db);
    const first = calls.batch;
    expect(first).toBeGreaterThan(0);
    await ensureSchema(db);
    await Promise.all([ensureSchema(db), ensureSchema(db)]);
    expect(calls.batch).toBe(first);
  });

  it("失敗は記憶せず、次の呼び出しでやり直す", async () => {
    let fail = true;
    const real = testDb();
    const db = {
      prepare: (q: string) => real.prepare(q),
      batch: (s: D1PreparedStatement[]) => {
        if (fail) return Promise.reject(new Error("boom"));
        return real.batch(s);
      },
    } as unknown as D1Database;
    await expect(ensureSchema(db)).rejects.toThrow("boom");
    fail = false;
    await expect(ensureSchema(db)).resolves.toBeUndefined();
  });
});
