/**
 * issue #35 のレビュー指摘への対応。
 * 1. 版チェックは include フラグを受けるリソースだけ(invoices / payments / users / project_costs は版を無視する)
 * 2. prune できなかった full では query_version を進めない
 * 3. 書類の逆引き走査(document_scan_state)にも版を持たせる
 * 4. cacheInfoFromState の partial 判定に deltaInProgress も含める
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LIST_FILTER_SUPPORT } from "../../src/board/openapi-support";
import { cacheReport } from "../../src/cache/admin";
import { getScanState } from "../../src/cache/document-map";
import { cacheInfoFromState } from "../../src/cache/info";
import { applySchema } from "../../src/cache/schema";
import {
  CACHE_QUERY_VERSION,
  deltaInProgress,
  fullInProgress,
  getSyncState,
  hasSynced,
} from "../../src/cache/sync-state";
import { type RefreshTarget, refreshResource } from "../../src/refresh/refresher";
import {
  COMPLETE_TTL_MS,
  createDocumentIndex,
  SCAN_LOCK_STALE_MS,
} from "../../src/repository/document-index";
import { SPECS } from "../../src/repository/resources";
import { BASE, makeEntities, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const NO_ARCHIVE = ["invoices", "payments", "project_costs", "users"];
const ARCHIVE = Object.entries(LIST_FILTER_SUPPORT)
  .filter(([, s]) => s.includeArchiveFlg)
  .map(([n]) => n);

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

function setup() {
  const clock = fakeClock();
  const db = testDb();
  const deps = { db, client: testClient(clock.now), now: clock.now, pageBudgetFloorMs: 0 };
  return { clock, db, deps };
}

/** query_version NULL(旧コード)で同期済みの行。 */
async function seedSynced(db: D1Database, resource: string, extra = "") {
  await db.batch([
    db.prepare("INSERT OR IGNORE INTO sync_state (resource_name) VALUES (?)").bind(resource),
    db
      .prepare(
        `UPDATE sync_state SET last_synced_at='2026-11-04T00:00:00Z', last_full_synced_at='2026-11-01T00:00:00Z',
         last_sync_mode='full', last_sync_status='success', cursor_updated_at='2026-11-03T00:00:00.000+09:00',
         full_generation = 1 ${extra} WHERE resource_name = ?`,
      )
      .bind(resource),
  ]);
}

describe("1. 版チェックは include フラグを受けるリソースだけ", () => {
  it("対象の確認", () => {
    expect(ARCHIVE).toHaveLength(14);
  });

  it.each(NO_ARCHIVE)(
    "%s: query_version が NULL でも同期済み・進捗は再開可能",
    async (resource) => {
      const db = testDb();
      await seedSynced(db, resource, ", full_next_page = 3, delta_next_page = 2");
      const st = await getSyncState(db, resource);
      expect(st?.query_version).toBeNull();
      expect(hasSynced(st)).toBe(true);
      expect(fullInProgress(st)).toBe(true);
      expect(deltaInProgress(st)).toBe(true);
      // 表示も同期済み
      const info = cacheInfoFromState(resource, st);
      expect(info.cached_at).toBe("2026-11-04T00:00:00Z");
      expect(info.refresh_state).toBe("partial");
    },
  );

  it.each(ARCHIVE)("%s: query_version が NULL なら未同期・進捗は再開しない", async (resource) => {
    const db = testDb();
    await seedSynced(db, resource, ", full_next_page = 3, delta_next_page = 2");
    const st = await getSyncState(db, resource);
    expect(hasSynced(st)).toBe(false);
    expect(fullInProgress(st)).toBe(false);
    expect(deltaInProgress(st)).toBe(false);
  });

  it("cache_status: 非対応リソースは complete、対応リソースは not_synced", async () => {
    const db = testDb();
    await seedSynced(db, "users");
    await seedSynced(db, "clients");
    const rep = await cacheReport(db);
    const by = (n: string) => rep.resources.find((r) => r.resource === n);
    expect(by("users")?.refresh_state).toBe("complete");
    expect(by("users")?.cached_at).toBe("2026-11-04T00:00:00Z");
    expect(by("clients")?.refresh_state).toBe("not_synced");
    expect(by("clients")?.cached_at).toBeUndefined();
  });

  it("非対応リソースの delta は、query_version が NULL でも delta のまま動く", async () => {
    const { db, deps } = setup();
    const target = targetOf("invoices");
    await seedSynced(db, "invoices");
    const api = mockOutboundFetch({
      [`GET ${BASE}${target.path}`]: pagedHandler(() => makeEntities(2)),
    });
    const out = await refreshResource(deps, target, "delta");
    expect(out.mode).toBe("delta");
    expect(api.calls[0]?.url).toContain("updated_at_gteq");
  });

  it("非対応リソースの途中の delta は、旧コードの進捗(delta_query_version NULL)から再開する", async () => {
    const { db, deps, clock } = setup();
    const target = targetOf("invoices");
    await seedSynced(
      db,
      "invoices",
      `, delta_next_page = 3, delta_started_at = '${new Date(clock.now() - 1000).toISOString()}'`,
    );
    const api = mockOutboundFetch({
      [`GET ${BASE}${target.path}`]: pagedHandler(() => makeEntities(500)),
    });
    await refreshResource(deps, target, "delta");
    // 1 ページ重ねて page 2 から
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("page")).toBe("2");
  });

  it("対応リソースの delta は、query_version が NULL なら full になる(従来どおり)", async () => {
    const { db, deps } = setup();
    const target = targetOf("clients");
    await seedSynced(db, "clients");
    mockOutboundFetch({ [`GET ${BASE}${target.path}`]: pagedHandler(() => makeEntities(2)) });
    const out = await refreshResource(deps, target, "delta");
    expect(out.mode).toBe("full");
  });
});

describe("2. prune できなかった full では query_version を進めない", () => {
  const clients = targetOf("clients");
  const KEY = `GET ${BASE}${clients.path}`;

  it("版 NULL の行に、件数が total に届かない full: complete でも query_version は NULL・未同期・次の delta は full", async () => {
    const { db, deps, clock } = setup();
    await seedSynced(db, "clients");
    // 3 件返すが X-Total-Count は 10(取りこぼし)
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
    });
    const out = await refreshResource(deps, clients, "full");
    expect(out.state).toBe("complete");
    expect(out.pruneSkipped).toBe(true);
    const st = await getSyncState(db, "clients");
    expect(st?.full_prune_skipped).toBe(1);
    expect(st?.query_version).toBeNull();
    expect(hasSynced(st)).toBe(false);

    vi.restoreAllMocks();
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(3)) });
    const next = await refreshResource(deps, clients, "delta");
    expect(next.mode).toBe("full");
  });

  it("届いた full では query_version が CACHE_QUERY_VERSION になる", async () => {
    const { db, deps } = setup();
    await seedSynced(db, "clients");
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: true }),
    });
    await refreshResource(deps, clients, "full");
    const st = await getSyncState(db, "clients");
    expect(st?.full_prune_skipped).toBe(0);
    expect(st?.query_version).toBe(CACHE_QUERY_VERSION);
    expect(hasSynced(st)).toBe(true);
  });

  it("すでに版が一致している行は、prune できなくても版は保たれる(定常は従来どおり)", async () => {
    const { db, deps } = setup();
    await seedSynced(db, "clients", `, query_version = ${CACHE_QUERY_VERSION}`);
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), undefined, { totalHeader: 10 }),
    });
    await refreshResource(deps, clients, "full");
    expect((await getSyncState(db, "clients"))?.query_version).toBe(CACHE_QUERY_VERSION);
  });
});

describe("3. 書類の逆引き走査に版を持たせる", () => {
  const projectsKey = `GET ${BASE}/v1/projects`;
  const PER_PAGE = 2;
  const projects = (n: number) =>
    Array.from({ length: n }, (_, i) => ({
      id: i + 1,
      name: `案件${i + 1}`,
      estimate: { id: 1000 + i + 1 },
    }));

  function scanSetup() {
    const clock = fakeClock();
    const db = testDb();
    const index = createDocumentIndex({
      db,
      client: testClient(clock.now),
      now: clock.now,
      perPage: PER_PAGE,
    });
    return { clock, db, index };
  }
  const pagesOf = (calls: { url: string }[]) =>
    calls.map((c) => Number(new URL(c.url).searchParams.get("page")));

  it("旧版の途中の進捗(next_page=3、版 NULL)は再開せず page 1 から走査する", async () => {
    const { db, index, clock } = scanSetup();
    await db
      .prepare(
        "INSERT INTO document_scan_state (document_kind, next_page, started_at) VALUES ('estimate', 3, ?)",
      )
      .bind(new Date(clock.now() - 1000).toISOString())
      .run();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(5)) });
    const r = await index.lookup("estimate", 1001);
    expect(r).toEqual({ status: "hit", projectId: 1 });
    expect(pagesOf(api.calls)[0]).toBe(1);
    expect((await getScanState(db, "estimate"))?.query_version).toBe(CACHE_QUERY_VERSION);
  });

  it("旧版の完了済み(TTL 内)でも走査し直す", async () => {
    const { db, index, clock } = scanSetup();
    await db
      .prepare(
        "INSERT INTO document_scan_state (document_kind, completed_at, started_at) VALUES ('estimate', ?, ?)",
      )
      .bind(new Date(clock.now() - 1000).toISOString(), new Date(clock.now() - 5000).toISOString())
      .run();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(5)) });
    // アーカイブ案件の書類(5 件目)が hit する
    expect(await index.lookup("estimate", 1005)).toEqual({ status: "hit", projectId: 5 });
    expect(api.calls.length).toBeGreaterThan(0);
  });

  it("新版の進捗は従来どおり再開する", async () => {
    const { db, index, clock } = scanSetup();
    await db
      .prepare(
        "INSERT INTO document_scan_state (document_kind, next_page, started_at, query_version) VALUES ('estimate', 3, ?, ?)",
      )
      .bind(new Date(clock.now() - 1000).toISOString(), CACHE_QUERY_VERSION)
      .run();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(6)) });
    await index.lookup("estimate", 1005);
    expect(pagesOf(api.calls)[0]).toBe(3);
  });

  it("新版の完了済みは TTL 内 not_found(通信しない)。TTL 後は走査し直す", async () => {
    const { db, index, clock } = scanSetup();
    mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(3)) });
    expect(await index.lookup("estimate", 99999)).toEqual({ status: "miss", reason: "not_found" });
    vi.restoreAllMocks();
    const api = mockOutboundFetch({});
    expect(await index.lookup("estimate", 99999)).toEqual({ status: "miss", reason: "not_found" });
    expect(api.calls).toHaveLength(0);
    expect((await getScanState(db, "estimate"))?.query_version).toBe(CACHE_QUERY_VERSION);
    clock.advance(COMPLETE_TTL_MS + 1000);
    vi.restoreAllMocks();
    const api2 = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(3)) });
    await index.lookup("estimate", 99999);
    expect(api2.calls.length).toBeGreaterThan(0);
  });

  it("scan のロックを奪われた旧保持者は版を書けない", async () => {
    const { db, index, clock } = scanSetup();
    mockOutboundFetch({
      [projectsKey]: pagedHandler(
        () => projects(5),
        async () => {
          // 走査中に stale として奪われる
          clock.advance(SCAN_LOCK_STALE_MS + 1_000);
          await db
            .prepare(
              "UPDATE document_scan_state SET lock_owner = 'B', lock_started_at = ? WHERE document_kind = 'estimate'",
            )
            .bind(new Date(clock.now()).toISOString())
            .run();
          return undefined;
        },
      ),
    });
    const r = await index.lookup("estimate", 99999, { deadline: clock.now() + 3_600_000 });
    expect(r).toEqual({ status: "unresolved", reason: "scan_in_progress" });
    const st = await getScanState(db, "estimate");
    expect(st?.query_version).toBeNull();
    expect(st?.next_page).toBeNull();
    expect(st?.lock_owner).toBe("B");
  });

  it("cache_status の document_scans: 旧版は未開始相当(next_page などを出さない)、新版は従来どおり", async () => {
    const { db } = scanSetup();
    await db.batch([
      db.prepare(
        "INSERT INTO document_scan_state (document_kind, next_page, started_at) VALUES ('estimate', 3, '2026-11-04T00:00:00Z')",
      ),
      db.prepare(
        "INSERT INTO document_scan_state (document_kind, completed_at, started_at) VALUES ('order', '2026-11-04T00:00:00Z', '2026-11-04T00:00:00Z')",
      ),
      db
        .prepare(
          "INSERT INTO document_scan_state (document_kind, next_page, started_at, query_version) VALUES ('delivery', 4, '2026-11-04T00:00:00Z', ?)",
        )
        .bind(CACHE_QUERY_VERSION),
    ]);
    const { document_scans } = await cacheReport(db);
    const by = (k: string) => document_scans.find((d) => d.document_kind === k);
    expect(by("estimate")).toMatchObject({ scan_state: "not_started" });
    expect(by("estimate")?.next_page).toBeUndefined();
    expect(by("estimate")?.started_at).toBeUndefined();
    expect(by("order")).toMatchObject({ scan_state: "not_started" });
    expect(by("order")?.completed_at).toBeUndefined();
    expect(by("delivery")).toMatchObject({ scan_state: "in_progress", next_page: 4 });
  });

  it("移行 v4: document_scan_state に query_version を足す。既存の行は壊れず NULL、冪等", async () => {
    const db = testDb();
    await applySchema(db);
    await db.batch([
      db.prepare("ALTER TABLE document_scan_state DROP COLUMN query_version"),
      db.prepare(
        "INSERT OR REPLACE INTO document_scan_state (document_kind, next_page) VALUES ('estimate', 3)",
      ),
      db.prepare("UPDATE cache_meta SET value = '3' WHERE key = 'db_schema_version'"),
    ]);
    await applySchema(db);
    const row = await db
      .prepare("SELECT * FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<Record<string, unknown>>();
    expect(row).toMatchObject({ next_page: 3, query_version: null });
    await db.prepare("UPDATE cache_meta SET value = '3' WHERE key = 'db_schema_version'").run();
    await applySchema(db);
    await applySchema(db);
    const cols = (
      await db.prepare("PRAGMA table_info(document_scan_state)").all<{ name: string }>()
    ).results;
    expect(cols.filter((c) => c.name === "query_version")).toHaveLength(1);
  });
});

describe("4. cacheInfoFromState の partial 判定に deltaInProgress も含める", () => {
  it("delta が途中(現在の版)なら partial。旧版の delta の進捗は partial にしない", async () => {
    const db = testDb();
    await seedSynced(
      db,
      "clients",
      `, query_version = ${CACHE_QUERY_VERSION}, delta_next_page = 2`,
    );
    // delta_query_version が NULL(旧コード)
    expect(cacheInfoFromState("clients", await getSyncState(db, "clients")).refresh_state).toBe(
      undefined,
    );
    await db
      .prepare("UPDATE sync_state SET delta_query_version = ? WHERE resource_name = 'clients'")
      .bind(CACHE_QUERY_VERSION)
      .run();
    expect(cacheInfoFromState("clients", await getSyncState(db, "clients")).refresh_state).toBe(
      "partial",
    );
  });
});
