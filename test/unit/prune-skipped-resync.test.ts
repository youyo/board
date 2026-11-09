/**
 * issue #40: prune を見送った full(件数が BOARD の X-Total-Count に届かない)は「同期成功」ではなく、
 * must_full_resync=1 を立てて、次の refresh=delta を full として動かす。件数照合が通る full で 0 に戻る。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildClientsQuery } from "../../src/board/resources/clients";
import { CACHE_QUERY_VERSION, getSyncState, hasSynced } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { LOCK_STALE_MS, tryAcquireLock } from "../../src/refresh/lock";
import {
  MUST_FULL_RESYNC_MIN_INTERVAL_MS,
  type RefreshTarget,
  refreshResource,
} from "../../src/refresh/refresher";
import { BASE, makeEntities, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const clients: RefreshTarget = {
  resource: "clients",
  path: "/v1/clients",
  buildQuery: buildClientsQuery,
};
const SIX_HOURS = 6 * 60 * 60_000;
const KEY = `GET ${BASE}/v1/clients`;

function setup() {
  const clock = fakeClock();
  const db = testDb();
  const deps = { db, client: testClient(clock.now), now: clock.now, pageBudgetFloorMs: 0 };
  return { clock, db, deps };
}

describe("prune を見送った full は must_full_resync を立てる", () => {
  it("件数が届かない full の完了: full_prune_skipped=1 かつ must_full_resync=1(同期済みの読み取りは続けられる)", async () => {
    const { db, deps, clock } = setup();
    // 先に件数の合う full で同期済みにする(版は照合が通った full だけが進める)
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();
    clock.advance(60_000);
    // 3 件返すが BOARD の総件数は 10
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
    });
    const out = await refreshResource(deps, clients, "full");
    expect(out).toMatchObject({ state: "complete", pruneSkipped: true });
    const st = await getSyncState(db, "clients");
    expect(st?.full_prune_skipped).toBe(1);
    expect(st?.must_full_resync).toBe(1);
    // 読み取りの hasSynced は変えない(欠けは一部で、読みは続けてよい)
    expect(hasSynced(st)).toBe(true);
  });

  it("次の refresh=delta は full として動き(updated_at_gteq を送らない)、件数が合えば must_full_resync=0 に戻る", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
    });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();

    // 前回の full の完了から間隔(6 時間)が空いたので、delta は full に切り替わる
    clock.advance(MUST_FULL_RESYNC_MIN_INTERVAL_MS);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({ mode: "full", state: "complete" });
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
    const st = await getSyncState(db, "clients");
    expect(st?.must_full_resync).toBe(0);
    expect(st?.full_prune_skipped).toBe(0);
    vi.restoreAllMocks();

    // 戻ったあとの delta は delta
    clock.advance(60_000);
    const api2 = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(1)) });
    const out2 = await refreshResource(deps, clients, "delta");
    expect(out2.mode).toBe("delta");
    expect(api2.calls[0]?.url).toContain("updated_at_gteq");
  });

  it("full を繰り返しても件数が合わなければ、間隔ごとに full のまま(must_full_resync=1 が続く)", async () => {
    const { db, deps, clock } = setup();
    for (let i = 0; i < 3; i++) {
      mockOutboundFetch({
        [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
      });
      const out = await refreshResource(deps, clients, "delta");
      expect(out.mode).toBe("full");
      expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(1);
      vi.restoreAllMocks();
      clock.advance(MUST_FULL_RESYNC_MIN_INTERVAL_MS);
    }
  });

  it("件数が合う通常の full は must_full_resync=0 のまま(次の delta は delta)", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    await refreshResource(deps, clients, "full");
    expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(0);
    vi.restoreAllMocks();
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(1)) });
    expect((await refreshResource(deps, clients, "delta")).mode).toBe("delta");
  });

  it("X-Total-Count がない(null)full は照合が通る扱いで must_full_resync=0", async () => {
    const { db, deps } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3)) });
    await refreshResource(deps, clients, "full");
    expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(0);
  });

  it("版 1 の定常状態(query_version が現行・カーソルあり)でも、must_full_resync=1 なら delta は full になる", async () => {
    const { db, deps } = setup();
    await db.batch([
      db.prepare("INSERT INTO sync_state (resource_name) VALUES ('clients')"),
      db
        .prepare(
          `UPDATE sync_state SET last_synced_at = '2026-11-04T00:00:00Z', query_version = ?,
           cursor_updated_at = '2026-11-03T00:00:00.000+09:00', must_full_resync = 1 WHERE resource_name = 'clients'`,
        )
        .bind(CACHE_QUERY_VERSION),
    ]);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(2), undefined, { totalHeader: true }),
    });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("full");
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
    expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(0);
  });
});

describe("must_full_resync による full への切り替えは 6 時間に 1 回まで(rate limit の保護)", () => {
  /** 件数が届かない full で must_full_resync=1 にした状態を作る(その時刻が last_full_synced_at)。 */
  async function skippedState() {
    const ctx = setup();
    // 版(query_version)は照合が通った full だけが進めるので、先に件数の合う full で現行版にする
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    await refreshResource(ctx.deps, clients, "full");
    vi.restoreAllMocks();
    ctx.clock.advance(60_000);
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
    });
    await refreshResource(ctx.deps, clients, "full");
    vi.restoreAllMocks();
    expect((await getSyncState(ctx.db, "clients"))?.must_full_resync).toBe(1);
    return ctx;
  }

  it("前回の full の完了から 6 時間未満: delta は delta のまま(updated_at_gteq を送り、must_full_resync は立ったまま)", async () => {
    const { db, deps, clock } = await skippedState();
    clock.advance(SIX_HOURS - 1_000);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(1)) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("delta");
    expect(api.calls[0]?.url).toContain("updated_at_gteq");
    expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(1);
  });

  it("境界: ちょうど 6 時間で full に切り替わる(定数も 6 時間)", async () => {
    expect(MUST_FULL_RESYNC_MIN_INTERVAL_MS).toBe(SIX_HOURS);
    const { deps, clock } = await skippedState();
    clock.advance(SIX_HOURS);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("full");
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
  });

  it("6 時間以上: full に切り替わる", async () => {
    const { deps, clock } = await skippedState();
    clock.advance(SIX_HOURS + 60_000);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("full");
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
  });

  it("last_full_synced_at が NULL(または読めない)なら、間隔に関係なく full に切り替わる", async () => {
    for (const value of [null, "not-a-date"]) {
      await resetCache();
      const { db, deps } = setup();
      await db.batch([
        db.prepare("INSERT INTO sync_state (resource_name) VALUES ('clients')"),
        db
          .prepare(
            `UPDATE sync_state SET last_synced_at = '2026-11-04T00:00:00Z', query_version = ?,
             cursor_updated_at = '2026-11-03T00:00:00.000+09:00', must_full_resync = 1,
             last_full_synced_at = ? WHERE resource_name = 'clients'`,
          )
          .bind(CACHE_QUERY_VERSION, value),
      ]);
      const api = mockOutboundFetch({
        [KEY]: pagedHandler(() => makeEntities(2), undefined, { totalHeader: true }),
      });
      const out = await refreshResource(deps, clients, "delta");
      expect(out.mode).toBe("full");
      expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
      vi.restoreAllMocks();
    }
  });

  it("refresh=full は間隔に関係なく常に full", async () => {
    const { deps, clock } = await skippedState();
    clock.advance(60_000);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    const out = await refreshResource(deps, clients, "full");
    expect(out.mode).toBe("full");
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
  });

  it("切り替えで始まった full が partial で終わったら、次の delta も full の経路で、保存したページから再開する", async () => {
    const { db, deps, clock } = await skippedState();
    clock.advance(MUST_FULL_RESYNC_MIN_INTERVAL_MS);
    const data = makeEntities(450);
    // 1 ページ 4 秒、締め切り 10 秒: 2 ページで partial(次は 3)
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(4_000);
          return undefined;
        },
        { totalHeader: true },
      ),
    });
    const first = await refreshResource(deps, clients, "delta", { deadline: clock.now() + 10_000 });
    expect(first).toMatchObject({ mode: "full", state: "partial", nextPage: 3 });
    vi.restoreAllMocks();
    // partial は last_full_synced_at を進めない
    clock.advance(60_000);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => data, undefined, { totalHeader: true }),
    });
    const second = await refreshResource(deps, clients, "delta");
    expect(second).toMatchObject({ mode: "full", state: "complete" });
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
    // 最後に取り終えたページ(2)から取り直す(1 ページ重ねる)= 保存した進捗からの再開
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("page")).toBe("2");
    expect((await getSyncState(db, "clients"))?.must_full_resync).toBe(0);
  });
});

describe("stale ロックを奪われた旧保持者は must_full_resync を書けない", () => {
  it("full の最終ページ(件数が届かない)", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(3),
        async () => {
          clock.advance(LOCK_STALE_MS + 1_000);
          await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" });
          return undefined;
        },
        { totalHeader: 10 },
      ),
    });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "full", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    const st = await getSyncState(db, "clients");
    expect(st?.must_full_resync).toBe(0);
    expect(st?.full_prune_skipped).toBe(0);
  });
});
