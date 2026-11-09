/**
 * issue #35: refresh(full / delta)がアーカイブ済み・失注のレコードも取る(include_archive_flg=1 / include_lost_flg=1)。
 * あわせて、旧コードで作ったキャッシュ(欠けがある)を query_version で『未同期』として扱う。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LIST_FILTER_SUPPORT } from "../../src/board/openapi-support";
import { buildClientsQuery } from "../../src/board/resources/clients";
import { countEntries, getEntry } from "../../src/cache/resource-cache";
import { CACHE_QUERY_VERSION, getSyncState, hasSynced } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { LOCK_STALE_MS, tryAcquireLock } from "../../src/refresh/lock";
import { type RefreshTarget, refreshResource } from "../../src/refresh/refresher";
import { createRepositories } from "../../src/repository";
import { SPECS } from "../../src/repository/resources";
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
const KEY = `GET ${BASE}/v1/clients`;

function setup() {
  const clock = fakeClock();
  const db = testDb();
  const deps = { db, client: testClient(clock.now), now: clock.now, pageBudgetFloorMs: 0 };
  return { clock, db, deps };
}

function targetOf(resource: string): RefreshTarget {
  const spec = SPECS[resource as keyof typeof SPECS];
  if (!("buildQuery" in spec)) throw new Error(`no buildQuery: ${resource}`);
  return {
    resource,
    path: spec.path,
    buildQuery: (o, page, perPage) =>
      (spec.buildQuery as (o: object, p: number, pp: number) => string)(o, page, perPage),
  };
}

const ARCHIVE_RESOURCES = Object.entries(LIST_FILTER_SUPPORT)
  .filter(([, s]) => s.includeArchiveFlg)
  .map(([name]) => name);
const NO_ARCHIVE_RESOURCES = Object.entries(LIST_FILTER_SUPPORT)
  .filter(([, s]) => !s.includeArchiveFlg)
  .map(([name]) => name);

const flagsOf = (url: string) => {
  const q = new URL(url).searchParams;
  return { archive: q.get("include_archive_flg"), lost: q.get("include_lost_flg") };
};

/** 旧コードで同期済みの sync_state(query_version なし)を作る。 */
async function seedLegacySynced(db: D1Database, resource = "clients") {
  await db.batch([
    db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES (?)").bind(resource),
    db
      .prepare(
        `UPDATE sync_state SET last_synced_at = '2026-11-04T00:00:00Z', last_full_synced_at = '2026-11-04T00:00:00Z',
         last_sync_mode = 'full', last_sync_status = 'success', cursor_updated_at = '2026-11-03T00:00:00.000+09:00',
         full_generation = 1 WHERE resource_name = ?`,
      )
      .bind(resource),
  ]);
}

describe("refresh のクエリにアーカイブ・失注を含めるフラグが付く(全ページ)", () => {
  it("対象の確認: include_archive_flg を受ける 14 リソース、受けない 4 リソース", () => {
    expect(ARCHIVE_RESOURCES).toHaveLength(14);
    expect([...NO_ARCHIVE_RESOURCES].sort()).toEqual(
      ["invoices", "payments", "project_costs", "users"].sort(),
    );
  });

  function expectFlags(resource: string, urls: string[]) {
    expect(urls.length).toBeGreaterThanOrEqual(3); // 250 件 = 3 ページ
    for (const url of urls) {
      const f = flagsOf(url);
      if (ARCHIVE_RESOURCES.includes(resource)) {
        expect(f.archive, `${resource} ${url}`).toBe("1");
      } else {
        expect(f.archive, `${resource} ${url}`).toBeNull();
      }
      if (resource === "projects") expect(f.lost, url).toBe("1");
      else expect(f.lost, `${resource} ${url}`).toBeNull();
    }
  }

  const ALL = [...ARCHIVE_RESOURCES, ...NO_ARCHIVE_RESOURCES];

  it.each(ALL)("full: %s", async (resource) => {
    const { deps } = setup();
    const target = targetOf(resource);
    const api = mockOutboundFetch({
      [`GET ${BASE}${target.path}`]: pagedHandler(() => makeEntities(250)),
    });
    const out = await refreshResource(deps, target, "full");
    expect(out.state).toBe("complete");
    expectFlags(
      resource,
      api.calls.map((c) => c.url),
    );
  });

  it.each(ALL)("カーソルのない delta(= full として動く): %s", async (resource) => {
    const { deps } = setup();
    const target = targetOf(resource);
    const api = mockOutboundFetch({
      [`GET ${BASE}${target.path}`]: pagedHandler(() => makeEntities(250)),
    });
    const out = await refreshResource(deps, target, "delta");
    expect(out.mode).toBe("full");
    expectFlags(
      resource,
      api.calls.map((c) => c.url),
    );
  });

  it.each(ALL)("カーソルのある delta: %s", async (resource) => {
    const { deps, clock } = setup();
    const target = targetOf(resource);
    const key = `GET ${BASE}${target.path}`;
    mockOutboundFetch({ [key]: pagedHandler(() => makeEntities(5)) });
    await refreshResource(deps, target, "full");
    vi.restoreAllMocks();

    clock.advance(3_600_000);
    const api = mockOutboundFetch({ [key]: pagedHandler(() => makeEntities(250)) });
    await refreshResource(deps, target, "delta");
    expectFlags(
      resource,
      api.calls.map((c) => c.url),
    );
    if (LIST_FILTER_SUPPORT[resource]?.updatedAtGteq) {
      // 本当に delta として動いた(updated_at_gteq つき)
      expect(api.calls[0]?.url).toContain("updated_at_gteq");
    }
  });
});

describe("full の prune: アーカイブ済みの行は full 完了後も消えない", () => {
  it("別経路(get)で入ったアーカイブ済みの行が、BOARD の一覧(フラグつき)にも出れば世代が更新されて残る", async () => {
    const { db, deps } = setup();
    // 別経路で入った行(世代 0)
    await db
      .prepare(
        "INSERT INTO resource_cache (resource_name, entity_id, payload_json, updated_at, fetched_at, generation) VALUES ('clients', 3, ?, NULL, 'x', 0)",
      )
      .bind(JSON.stringify({ id: 3, name: "アーカイブ済み", archive_flg: 1 }))
      .run();
    // BOARD は include_archive_flg=1 のときだけアーカイブ済み(id=3)を返す
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => [],
        (_page, q) => {
          const all = makeEntities(3);
          const rows = q.get("include_archive_flg") === "1" ? all : all.filter((r) => r.id !== 3);
          return new Response(JSON.stringify(rows), {
            headers: { "Content-Type": "application/json", "X-Total-Count": String(rows.length) },
          });
        },
      ),
    });
    const out = await refreshResource(deps, clients, "full");
    expect(out).toMatchObject({ state: "complete" });
    expect(out.pruneSkipped).toBeUndefined();
    expect(await getEntry(db, "clients", 3)).not.toBeNull();
    expect(await countEntries(db, "clients")).toBe(3);
  });

  it("BOARD が本当に 0 件を返したときは、旧行が prune される", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => [], undefined, { totalHeader: true }) });
    const out = await refreshResource(deps, clients, "full");
    expect(out.state).toBe("complete");
    expect(await countEntries(db, "clients")).toBe(0);
  });
});

describe("query_version: 旧コードのキャッシュは未同期として扱い、取り直す", () => {
  it("完了した full / delta は query_version を CACHE_QUERY_VERSION にする", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3)) });
    await refreshResource(deps, clients, "full");
    let st = await getSyncState(db, "clients");
    expect(st?.query_version).toBe(CACHE_QUERY_VERSION);
    expect(hasSynced(st)).toBe(true);

    await db.prepare("UPDATE sync_state SET query_version = NULL").run();
    st = await getSyncState(db, "clients");
    expect(hasSynced(st)).toBe(false);
    vi.restoreAllMocks();
    // query_version が違う delta は full として動き、完了で版が入る
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3)) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("full");
    expect((await getSyncState(db, "clients"))?.query_version).toBe(CACHE_QUERY_VERSION);
  });

  it("hasSynced: last_synced_at があっても query_version が違えば false", async () => {
    const { db } = setup();
    await seedLegacySynced(db);
    const st = await getSyncState(db, "clients");
    expect(st?.last_synced_at).not.toBeNull();
    expect(st?.query_version).toBeNull();
    expect(hasSynced(st)).toBe(false);
  });

  it("既存行(query_version なし)に refresh=delta: カーソルがあっても full として動き、updated_at_gteq を送らない", async () => {
    const { db, deps } = setup();
    await seedLegacySynced(db);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3)) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({ mode: "full", state: "complete" });
    expect(api.calls[0]?.url).not.toContain("updated_at_gteq");
    expect(flagsOf(api.calls[0]?.url ?? "").archive).toBe("1");
    expect(hasSynced(await getSyncState(db, "clients"))).toBe(true);
  });

  it("旧版の full の進捗(full_query_version なしの full_next_page)は再開せず、1 ページ目から新しい世代で取る", async () => {
    const { db, deps, clock } = setup();
    await db.batch([
      db.prepare("INSERT INTO sync_state (resource_name) VALUES ('clients')"),
      db
        .prepare(
          `UPDATE sync_state SET full_generation = 2, full_next_page = 3, full_started_at = ?, last_sync_mode = 'full',
           last_sync_status = 'partial' WHERE resource_name = 'clients'`,
        )
        .bind(new Date(clock.now() - 60_000).toISOString().replace(/\.\d+Z$/, "Z")),
    ]);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(250)) });
    await refreshResource(deps, clients, "full");
    const pages = api.calls.map((c) => new URL(c.url).searchParams.get("page"));
    expect(pages).toEqual(["1", "2", "3"]);
    expect((await getSyncState(db, "clients"))?.full_generation).toBe(3);
  });

  it("full の進捗に版を保存し、同じ版の進捗は 1 ページ重ねて再開する", async () => {
    const { db, deps, clock } = setup();
    // 1 ページ取ったところで締め切り
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(250),
        (page) => {
          if (page === 2) clock.advance(3_600_000);
          return undefined;
        },
      ),
    });
    const first = await refreshResource(deps, clients, "full", {
      deadline: clock.now() + 1_000_000,
    });
    expect(first.state).toBe("partial");
    const st = await getSyncState(db, "clients");
    expect(st?.full_next_page).toBeGreaterThan(1);
    expect(st?.full_query_version).toBe(CACHE_QUERY_VERSION);
    expect(hasSynced(st)).toBe(false); // 完了するまで同期済みにならない
    vi.restoreAllMocks();

    clock.set(Date.parse("2026-11-05T00:10:00Z"));
    const api2 = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(250)) });
    await refreshResource(deps, clients, "full");
    expect(new URL(api2.calls[0]?.url ?? "").searchParams.get("page")).toBe(
      String(Math.max(1, (st?.full_next_page ?? 1) - 1)),
    );
  });

  it("旧版の delta の進捗(delta_query_version なし)は再開せず、1 ページ目から", async () => {
    const { db, deps, clock } = setup();
    await seedLegacySynced(db);
    await db
      .prepare(
        `UPDATE sync_state SET query_version = ?, delta_next_page = 3, delta_started_at = ?, delta_max_updated_at = NULL
         WHERE resource_name = 'clients'`,
      )
      .bind(
        CACHE_QUERY_VERSION,
        new Date(clock.now() - 60_000).toISOString().replace(/\.\d+Z$/, "Z"),
      )
      .run();
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(250)) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.mode).toBe("delta");
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["1", "2", "3"]);
  });

  it("同じ版の delta の進捗は 1 ページ重ねて再開する", async () => {
    const { db, deps, clock } = setup();
    await seedLegacySynced(db);
    await db
      .prepare(
        `UPDATE sync_state SET query_version = ?, delta_next_page = 3, delta_query_version = ?, delta_started_at = ?
         WHERE resource_name = 'clients'`,
      )
      .bind(
        CACHE_QUERY_VERSION,
        CACHE_QUERY_VERSION,
        new Date(clock.now() - 60_000).toISOString().replace(/\.\d+Z$/, "Z"),
      )
      .run();
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(250)) });
    await refreshResource(deps, clients, "delta");
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["2", "3"]);
  });

  it("完了後は絞り込みの読み取りがキャッシュに戻る(それまでは API)", async () => {
    const { db, deps } = setup();
    await seedLegacySynced(db);
    const repos = createRepositories({ db, client: testClient(deps.now), now: deps.now });
    const handler = pagedHandler(() => makeEntities(3));
    const api = mockOutboundFetch({ [KEY]: handler });

    const before = await repos.clients.list({ nameCont: "顧客" });
    expect(before.source).toBe("api");

    await refreshResource(deps, clients, "delta"); // 版が違うので full として動く
    const callsBefore = api.calls.length;
    const after = await repos.clients.list({ nameCont: "顧客" });
    expect(after.source).toBe("cache");
    expect(after.items).toHaveLength(3);
    expect(api.calls.length).toBe(callsBefore);
  });
});

describe("query_version: stale ロックを奪われた旧保持者は query_version を書けない", () => {
  function stolenDuringFetch(db: D1Database, clock: ReturnType<typeof fakeClock>) {
    return async () => {
      clock.advance(LOCK_STALE_MS + 1_000);
      await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" });
      return undefined;
    };
  }

  it("full の最終ページ", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3), stolenDuringFetch(db, clock)) });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "full", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    const st = await getSyncState(db, "clients");
    expect(st?.query_version).toBeNull();
    expect(hasSynced(st)).toBe(false);
  });

  it("delta の最終ページ", async () => {
    const { db, deps, clock } = setup();
    await db.batch([
      db.prepare("INSERT INTO sync_state (resource_name) VALUES ('clients')"),
      db
        .prepare(
          `UPDATE sync_state SET last_synced_at = '2026-11-04T00:00:00Z', query_version = ?, cursor_updated_at = '2026-11-03T00:00:00.000+09:00'
           WHERE resource_name = 'clients'`,
        )
        .bind(CACHE_QUERY_VERSION),
    ]);
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(2), stolenDuringFetch(db, clock)) });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "delta", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    // B は何も書いていない。A の完了 UPDATE は反映されない(last_synced_at は元のまま)。
    const st = await getSyncState(db, "clients");
    expect(st?.last_synced_at).toBe("2026-11-04T00:00:00Z");
    // 版も書けていない(delta の進捗の版は NULL のまま、query_version は元の値のまま)。
    expect(st?.delta_query_version).toBeNull();
    expect(st?.query_version).toBe(CACHE_QUERY_VERSION);
  });
});
