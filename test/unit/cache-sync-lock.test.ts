import { beforeEach, describe, expect, it } from "vitest";
import { getSyncState, hasSynced } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import {
  LOCK_STALE_MS,
  REFRESH_RETRY_AFTER_SECONDS,
  releaseLock,
  tryAcquireLock,
  withLock,
} from "../../src/refresh/lock";
import { fakeClock, resetCache, testDb } from "../helpers/d1";

beforeEach(async () => {
  await resetCache();
});

describe("TryLock(sync_state への条件付き UPDATE)", () => {
  it("空きなら取得でき、取得者と開始時刻が記録される", async () => {
    const db = testDb();
    const clock = fakeClock();
    const lock = await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    expect(lock.owner).toBe("A");
    const st = await getSyncState(db, "clients");
    expect(st?.refresh_owner).toBe("A");
    expect(st?.refresh_started_at).toBe("2026-11-05T00:00:00Z");
  });

  it("保持されている間は待たずに RefreshInProgressError(retry_after_seconds 30)", async () => {
    const db = testDb();
    const clock = fakeClock();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    clock.advance(3_000);
    const err = await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" }).catch((e) => e);
    expect(err).toBeInstanceOf(RefreshInProgressError);
    expect(err).toMatchObject({
      code: "refresh_in_progress",
      retryable: true,
      retryAfterSeconds: REFRESH_RETRY_AFTER_SECONDS,
      resource: "clients",
      holder: "A",
      elapsedSeconds: 3,
    });
    expect(REFRESH_RETRY_AFTER_SECONDS).toBe(30);
    expect(err.message).toContain("clients");
    // 取得者は変わっていない。
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBe("A");
  });

  it("別のリソースのロックは独立", async () => {
    const db = testDb();
    const clock = fakeClock();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    await expect(
      tryAcquireLock(db, "vendors", { now: clock.now, owner: "B" }),
    ).resolves.toBeTruthy();
  });

  it("10 分を超えた stale なロックは奪取できる。10 分ちょうどまでは奪えない", async () => {
    const db = testDb();
    const clock = fakeClock();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    expect(LOCK_STALE_MS).toBe(10 * 60_000);
    clock.advance(LOCK_STALE_MS - 1_000);
    await expect(
      tryAcquireLock(db, "clients", { now: clock.now, owner: "B" }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    clock.advance(2_000);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "C" });
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBe("C");
  });

  it("元の取得者は、奪われたあとのロックを解放できない(自分のロックだけ解放する)", async () => {
    const db = testDb();
    const clock = fakeClock();
    const a = await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    clock.advance(LOCK_STALE_MS + 1_000);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" });
    await releaseLock(db, "clients", a.owner);
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBe("B");
    await releaseLock(db, "clients", "B");
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
  });

  it("解放すると次の取得ができる", async () => {
    const db = testDb();
    const clock = fakeClock();
    const a = await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    await releaseLock(db, "clients", a.owner);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" });
  });

  it("同時に 2 つ取りに行っても、取得できるのは 1 つだけ", async () => {
    const db = testDb();
    const clock = fakeClock();
    const results = await Promise.allSettled([
      tryAcquireLock(db, "clients", { now: clock.now, owner: "A" }),
      tryAcquireLock(db, "clients", { now: clock.now, owner: "B" }),
      tryAcquireLock(db, "clients", { now: clock.now, owner: "C" }),
    ]);
    expect(results.filter((r) => r.status === "fulfilled")).toHaveLength(1);
    expect(results.filter((r) => r.status === "rejected")).toHaveLength(2);
  });

  it("ロックを取っただけでは『同期済み』にならない(キャッシュは空のまま)", async () => {
    const db = testDb();
    await tryAcquireLock(db, "clients", { now: fakeClock().now, owner: "A" });
    expect(hasSynced(await getSyncState(db, "clients"))).toBe(false);
  });
});

describe("withLock(finally で必ず解放)", () => {
  it("成功しても解放する", async () => {
    const db = testDb();
    const clock = fakeClock();
    const v = await withLock(db, "clients", { now: clock.now }, async () => 42);
    expect(v).toBe(42);
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
  });

  it("例外でも解放し、例外はそのまま伝わる", async () => {
    const db = testDb();
    const clock = fakeClock();
    await expect(
      withLock(db, "clients", { now: clock.now }, async () => {
        throw new Error("boom");
      }),
    ).rejects.toThrow("boom");
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "next" });
  });

  it("実行中は他から取れず、終わると取れる", async () => {
    const db = testDb();
    const clock = fakeClock();
    let inner: unknown;
    await withLock(db, "clients", { now: clock.now }, async () => {
      inner = await tryAcquireLock(db, "clients", { now: clock.now, owner: "X" }).catch((e) => e);
    });
    expect(inner).toBeInstanceOf(RefreshInProgressError);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "Y" });
  });

  it("取得できなかったときは fn を呼ばない", async () => {
    const db = testDb();
    const clock = fakeClock();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "A" });
    let called = false;
    await expect(
      withLock(db, "clients", { now: clock.now }, async () => {
        called = true;
      }),
    ).rejects.toBeInstanceOf(RefreshInProgressError);
    expect(called).toBe(false);
  });
});
