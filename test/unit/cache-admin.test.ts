import { beforeEach, describe, expect, it } from "vitest";
import { cacheStatus, clearCache, expireCache } from "../../src/cache/admin";
import { countEntries, upsertEntities } from "../../src/cache/resource-cache";
import { getSyncState } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { tryAcquireLock } from "../../src/refresh/lock";
import { fakeClock, resetCache, testDb } from "../helpers/d1";

const clock = fakeClock();
const NOW = clock.now();

async function seed(db: D1Database) {
  await upsertEntities(db, "clients", [{ id: 1 }, { id: 2 }], { now: NOW });
  await upsertEntities(db, "vendors", [{ id: 1 }], { now: NOW });
  await db.batch([
    db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES ('clients')"),
    db.prepare(
      "UPDATE sync_state SET last_synced_at='2026-11-04T00:00:00Z', last_full_synced_at='2026-11-01T00:00:00Z', last_sync_status='success' WHERE resource_name='clients'",
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

  it("未知のリソース名は argument_error", async () => {
    await expect(
      clearCache(testDb(), { resources: ["nope"], now: clock.now }),
    ).rejects.toMatchObject({
      code: "argument_error",
    });
  });
});
