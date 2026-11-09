import { beforeEach, describe, expect, it } from "vitest";
import {
  cacheReport,
  cacheStatus,
  clearCache,
  clearDocumentMap,
  expireCache,
} from "../../src/cache/admin";
import { recordStatements, tryAcquireScanLock } from "../../src/cache/document-map";
import { countEntries, upsertEntities } from "../../src/cache/resource-cache";
import { CACHE_QUERY_VERSION, getSyncState } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { LOCK_STALE_MS, tryAcquireLock } from "../../src/refresh/lock";
import { fakeClock, resetCache, testDb } from "../helpers/d1";

const clock = fakeClock();
const NOW = clock.now();

async function seed(db: D1Database) {
  await upsertEntities(db, "clients", [{ id: 1 }, { id: 2 }], { now: NOW });
  await upsertEntities(db, "vendors", [{ id: 1 }], { now: NOW });
  await db.batch([
    db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES ('clients')"),
    db.prepare(
      `UPDATE sync_state SET last_synced_at='2026-11-04T00:00:00Z', last_full_synced_at='2026-11-01T00:00:00Z', last_sync_status='success', query_version=${CACHE_QUERY_VERSION} WHERE resource_name='clients'`,
    ),
  ]);
}

beforeEach(async () => {
  await resetCache();
});

describe("cache status", () => {
  it("22 リソースすべてについて、件数・最終同期・full の進行・ロックの状態を返す", async () => {
    const db = testDb();
    await seed(db);
    const rows = await cacheStatus(db, { now: clock.now });
    expect(rows).toHaveLength(22);
    const clients = rows.find((r) => r.resource === "clients");
    expect(clients).toMatchObject({
      cached_rows: 2,
      last_synced_at: "2026-11-04T00:00:00Z",
      last_full_synced_at: "2026-11-01T00:00:00Z",
      last_sync_status: "success",
      consecutive_errors: 0,
      refreshing: false,
    });
    expect(rows.find((r) => r.resource === "vendors")).toMatchObject({ cached_rows: 1 });
    expect(rows.find((r) => r.resource === "users")).toMatchObject({ cached_rows: 0 });
    expect(rows.find((r) => r.resource === "users")?.last_synced_at).toBeUndefined();
  });

  it("full が途中のリソースは full_in_progress、ロック中は refreshing", async () => {
    const db = testDb();
    await tryAcquireLock(db, "users", { now: clock.now, owner: "A" });
    await db
      .prepare(
        "UPDATE sync_state SET full_generation=3, full_next_page=4, full_started_at='2026-11-05T00:00:00Z' WHERE resource_name='users'",
      )
      .run();
    const users = (await cacheStatus(db, { now: clock.now })).find((r) => r.resource === "users");
    expect(users).toMatchObject({
      refreshing: true,
      full_in_progress: { generation: 3, next_page: 4, started_at: "2026-11-05T00:00:00Z" },
    });
  });

  it("stale になったロックは refreshing に数えない", async () => {
    const db = testDb();
    await tryAcquireLock(db, "users", { now: clock.now, owner: "A" });
    const later = fakeClock(NOW + 11 * 60_000);
    const users = (await cacheStatus(db, { now: later.now })).find((r) => r.resource === "users");
    expect(users?.refreshing).toBe(false);
  });
});

describe("cache expire", () => {
  it("指定したリソースに expired_at と invalidate_reason=manual を付ける(キャッシュは消さない)", async () => {
    const db = testDb();
    await seed(db);
    await expireCache(db, { resources: ["clients"], now: clock.now });
    expect(await getSyncState(db, "clients")).toMatchObject({
      expired_at: "2026-11-05T00:00:00Z",
      invalidate_reason: "manual",
    });
    expect(await getSyncState(db, "vendors")).toBeNull();
    expect(await countEntries(db, "clients")).toBe(2);
  });

  it("resources を省略すると 22 リソースすべて", async () => {
    const db = testDb();
    await expireCache(db, { now: clock.now });
    expect(
      (await cacheStatus(db, { now: clock.now })).every(
        (r) => r.expired_at === "2026-11-05T00:00:00Z",
      ),
    ).toBe(true);
  });

  it("未知のリソース名は argument_error", async () => {
    await expect(
      expireCache(testDb(), { resources: ["nope"], now: clock.now }),
    ).rejects.toMatchObject({
      code: "argument_error",
    });
  });
});

describe("cache clear", () => {
  it("指定したリソースの行と同期状態を消す。ほかのリソースは残す", async () => {
    const db = testDb();
    await seed(db);
    await clearCache(db, { resources: ["clients"], now: clock.now });
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await getSyncState(db, "clients")).toBeNull();
    expect(await countEntries(db, "vendors")).toBe(1);
  });

  it("resources を省略すると全リソース", async () => {
    const db = testDb();
    await seed(db);
    await clearCache(db, { now: clock.now });
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await countEntries(db, "vendors")).toBe(0);
  });

  it("refresh 中(ロック保持)のリソースは消さず RefreshInProgressError", async () => {
    const db = testDb();
    await seed(db);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    await expect(clearCache(db, { now: clock.now })).rejects.toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(2);
    expect(await countEntries(db, "vendors")).toBe(1);
  });

  it("ロックを取ってから消す: 削除の直前に別の取得を試みても弾かれ、ロックは clear の完了後に残らない", async () => {
    const db = testDb();
    await seed(db);
    let inner: unknown;
    await clearCache(db, {
      resources: ["clients"],
      now: clock.now,
      hooks: {
        afterLock: async () => {
          inner = await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" }).catch(
            (e) => e,
          );
        },
      },
    });
    expect(inner).toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await getSyncState(db, "clients")).toBeNull();
  });

  it("確認から削除までの間に stale ロックを別の取得者に奪われたら、何も消さず、その人のロック行を残す", async () => {
    const db = testDb();
    await seed(db);
    // 時計を進めて、clear が取ったロックを stale にして B に奪わせる。
    const local = fakeClock();
    await expect(
      clearCache(db, {
        resources: ["clients"],
        now: local.now,
        hooks: {
          afterLock: async () => {
            local.advance(LOCK_STALE_MS + 1_000);
            await tryAcquireLock(db, "clients", { now: local.now, owner: "B" });
          },
        },
      }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(2);
    expect(await getSyncState(db, "clients")).toMatchObject({ refresh_owner: "B" });
  });

  it("複数リソースのうち 1 つのロックを奪われたら、どのリソースも消さない(全か無か)", async () => {
    const db = testDb();
    await seed(db);
    const local = fakeClock();
    await expect(
      clearCache(db, {
        resources: ["clients", "vendors"],
        now: local.now,
        hooks: {
          afterLock: async () => {
            // 時計を進めて vendors のロックだけを stale にして B に奪わせる。
            local.advance(LOCK_STALE_MS + 1_000);
            await tryAcquireLock(db, "vendors", { now: local.now, owner: "B" });
          },
        },
      }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(2);
    expect(await countEntries(db, "vendors")).toBe(1);
    expect(await getSyncState(db, "clients")).not.toBeNull();
    expect(await getSyncState(db, "vendors")).toMatchObject({ refresh_owner: "B" });
  });

  it("複数リソースの途中で busy が見つかったら、それまでに取ったロックを返して何も消さない", async () => {
    const db = testDb();
    await seed(db);
    await tryAcquireLock(db, "vendors", { now: clock.now, owner: "A" });
    await expect(
      clearCache(db, { resources: ["clients", "vendors"], now: clock.now }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(2);
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
    expect((await getSyncState(db, "vendors"))?.refresh_owner).toBe("A");
  });

  it("未知のリソース名は argument_error", async () => {
    await expect(
      clearCache(testDb(), { resources: ["nope"], now: clock.now }),
    ).rejects.toMatchObject({
      code: "argument_error",
    });
  });
});

async function seedDocumentMap(db: D1Database) {
  await db.batch([
    ...recordStatements(
      db,
      "estimate",
      [
        { documentId: 10, projectId: 1 },
        { documentId: 11, projectId: 2 },
      ],
      NOW,
    ),
    ...recordStatements(db, "order", [{ documentId: 20, projectId: 1 }], NOW),
    db.prepare(
      `INSERT INTO document_scan_state (document_kind, next_page, started_at, query_version) VALUES ('estimate', 4, '2026-11-04T00:00:00Z', ${CACHE_QUERY_VERSION})`,
    ),
    db.prepare(
      `INSERT INTO document_scan_state (document_kind, next_page, started_at, completed_at, query_version) VALUES ('order', NULL, '2026-11-03T00:00:00Z', '2026-11-03T00:01:00Z', ${CACHE_QUERY_VERSION})`,
    ),
  ]);
}

describe("cache report (cache_status の元)", () => {
  it("22 リソースを cached_at / full_refreshed_at / refresh_state / row_count / full_prune_skipped / locked で返す", async () => {
    const db = testDb();
    await seed(db);
    const { resources } = await cacheReport(db, { now: clock.now });
    expect(resources).toHaveLength(22);
    const clients = resources.find((r) => r.resource === "clients");
    expect(clients).toEqual({
      resource: "clients",
      cached_at: "2026-11-04T00:00:00Z",
      full_refreshed_at: "2026-11-01T00:00:00Z",
      refresh_state: "complete",
      row_count: 2,
      full_prune_skipped: false,
      locked: false,
    });
    // 未取得のリソースは cached_at / full_refreshed_at を出さず、refresh_state は not_synced。
    expect(resources.find((r) => r.resource === "users")).toEqual({
      resource: "users",
      refresh_state: "not_synced",
      row_count: 0,
      full_prune_skipped: false,
      locked: false,
    });
  });

  it("full が途中なら partial、直近が失敗なら failed(エラーコード付き)、照合に通らず削除を見送ったら full_prune_skipped", async () => {
    const db = testDb();
    await db.batch([
      db.prepare(
        `INSERT INTO sync_state (resource_name, last_synced_at, query_version, full_next_page, full_query_version) VALUES ('clients', '2026-11-04T00:00:00Z', ${CACHE_QUERY_VERSION}, 3, ${CACHE_QUERY_VERSION})`,
      ),
      db.prepare(
        `INSERT INTO sync_state (resource_name, last_synced_at, query_version, last_sync_status, last_error_code, consecutive_failures, full_prune_skipped) VALUES ('vendors', '2026-11-04T00:00:00Z', ${CACHE_QUERY_VERSION}, 'error', 'rate_limited', 2, 1)`,
      ),
    ]);
    const { resources } = await cacheReport(db, { now: clock.now });
    expect(resources.find((r) => r.resource === "clients")?.refresh_state).toBe("partial");
    expect(resources.find((r) => r.resource === "vendors")).toMatchObject({
      refresh_state: "failed",
      last_error_code: "rate_limited",
      full_prune_skipped: true,
    });
  });

  it("delta が途中(delta_next_page が残っている)なら partial(続きを取る必要があるとクライアントに伝える)", async () => {
    const db = testDb();
    await db.batch([
      db.prepare(
        `INSERT INTO sync_state (resource_name, last_synced_at, query_version, last_sync_status, delta_next_page, delta_query_version, delta_started_at) VALUES ('projects', '2026-11-04T00:00:00Z', ${CACHE_QUERY_VERSION}, 'success', 2, ${CACHE_QUERY_VERSION}, '2026-11-05T00:00:00Z')`,
      ),
    ]);
    const { resources } = await cacheReport(db, { now: clock.now });
    expect(resources.find((r) => r.resource === "projects")?.refresh_state).toBe("partial");
  });

  it("ロック中は locked、stale なら locked ではない", async () => {
    const db = testDb();
    await tryAcquireLock(db, "users", { now: clock.now, owner: "A" });
    expect(
      (await cacheReport(db, { now: clock.now })).resources.find((r) => r.resource === "users")
        ?.locked,
    ).toBe(true);
    const later = fakeClock(NOW + LOCK_STALE_MS + 1_000);
    expect(
      (await cacheReport(db, { now: later.now })).resources.find((r) => r.resource === "users")
        ?.locked,
    ).toBe(false);
  });

  it("逆引きの走査状態を書類種別(4 種)ごとに返す", async () => {
    const db = testDb();
    await seedDocumentMap(db);
    const { document_scans } = await cacheReport(db, { now: clock.now });
    expect(document_scans.map((d) => d.document_kind)).toEqual([
      "estimate",
      "order",
      "delivery",
      "receipt",
    ]);
    expect(document_scans[0]).toEqual({
      document_kind: "estimate",
      scan_state: "in_progress",
      mapped_rows: 2,
      next_page: 4,
      started_at: "2026-11-04T00:00:00Z",
      locked: false,
    });
    expect(document_scans[1]).toMatchObject({
      document_kind: "order",
      scan_state: "complete",
      mapped_rows: 1,
      completed_at: "2026-11-03T00:01:00Z",
    });
    expect(document_scans[2]).toEqual({
      document_kind: "delivery",
      scan_state: "not_started",
      mapped_rows: 0,
      locked: false,
    });
  });

  it("読み取り専用: 状態を変えない", async () => {
    const db = testDb();
    await seed(db);
    await cacheReport(db, { now: clock.now });
    expect(await countEntries(db, "clients")).toBe(2);
    expect(await getSyncState(db, "users")).toBeNull();
  });
});

describe("clearDocumentMap", () => {
  it("逆引きの対応表と走査状態を全種別について消す", async () => {
    const db = testDb();
    await seedDocumentMap(db);
    await clearDocumentMap(db, { now: clock.now });
    const maps = await db
      .prepare("SELECT count(*) AS c FROM document_project_map")
      .first<{ c: number }>();
    const scans = await db
      .prepare("SELECT count(*) AS c FROM document_scan_state")
      .first<{ c: number }>();
    expect(maps?.c).toBe(0);
    expect(scans?.c).toBe(0);
  });

  it("走査中(ロック保持)の種別があれば何も消さず RefreshInProgressError。取ったロックは返す", async () => {
    const db = testDb();
    await seedDocumentMap(db);
    await tryAcquireScanLock(db, "receipt", "other", NOW, 120_000);
    await expect(clearDocumentMap(db, { now: clock.now })).rejects.toBeInstanceOf(
      RefreshInProgressError,
    );
    const maps = await db
      .prepare("SELECT count(*) AS c FROM document_project_map")
      .first<{ c: number }>();
    expect(maps?.c).toBe(3);
    const owners = await db
      .prepare(
        "SELECT document_kind, lock_owner FROM document_scan_state WHERE lock_owner IS NOT NULL",
      )
      .all<{ document_kind: string; lock_owner: string }>();
    expect(owners.results).toEqual([{ document_kind: "receipt", lock_owner: "other" }]);
  });

  it("確認から削除までの間にロックを奪われたら、何も消さない(所有者条件つきの batch)", async () => {
    const db = testDb();
    await seedDocumentMap(db);
    const local = fakeClock();
    await expect(
      clearDocumentMap(db, {
        now: local.now,
        hooks: {
          afterLock: async () => {
            local.advance(10 * 60_000);
            await tryAcquireScanLock(db, "estimate", "thief", local.now(), 120_000);
          },
        },
      }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    const maps = await db
      .prepare("SELECT count(*) AS c FROM document_project_map WHERE document_kind = 'estimate'")
      .first<{ c: number }>();
    expect(maps?.c).toBe(2);
    // 全か無か: 奪われていない種別(order)の対応表と走査状態も消えない。
    const all = await db
      .prepare("SELECT count(*) AS c FROM document_project_map")
      .first<{ c: number }>();
    expect(all?.c).toBe(3);
    const orderScan = await db
      .prepare("SELECT count(*) AS c FROM document_scan_state WHERE document_kind = 'order'")
      .first<{ c: number }>();
    expect(orderScan?.c).toBe(1);
    const row = await db
      .prepare("SELECT lock_owner FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ lock_owner: string }>();
    expect(row?.lock_owner).toBe("thief");
  });
});

/**
 * D1 は 1 文あたりのバインド変数を 100 までに制限する(Cloudflare D1 Limits)。ローカルの SQLite(miniflare)は
 * 上限が大きく、超えても他のテストでは検知できないので、全リソースを対象にした削除のバインド数をここで固定する。
 */
const D1_MAX_BOUND_PARAMS = 100;

function recordingDb(db: D1Database): { db: D1Database; bindCounts: number[] } {
  const bindCounts: number[] = [];
  const wrapped = new Proxy(db, {
    get(target, prop, receiver) {
      if (prop === "prepare") {
        return (sql: string) => {
          const stmt = target.prepare(sql);
          const bind = stmt.bind.bind(stmt);
          stmt.bind = (...values: unknown[]) => {
            bindCounts.push(values.length);
            return bind(...values);
          };
          return stmt;
        };
      }
      const value = Reflect.get(target, prop, receiver);
      return typeof value === "function" ? value.bind(target) : value;
    },
  });
  return { db: wrapped, bindCounts };
}

describe("D1 のバインド変数の上限", () => {
  it("全リソースを対象にした clearCache の各文はバインド変数が 100 以下", async () => {
    const { db, bindCounts } = recordingDb(testDb());
    await clearCache(db, { now: clock.now });
    expect(bindCounts.length).toBeGreaterThan(0);
    expect(Math.max(...bindCounts)).toBeLessThanOrEqual(D1_MAX_BOUND_PARAMS);
  });

  it("clearDocumentMap の各文はバインド変数が 100 以下", async () => {
    const { db, bindCounts } = recordingDb(testDb());
    await clearDocumentMap(db, { now: clock.now });
    expect(bindCounts.length).toBeGreaterThan(0);
    expect(Math.max(...bindCounts)).toBeLessThanOrEqual(D1_MAX_BOUND_PARAMS);
  });
});
