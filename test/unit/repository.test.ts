import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearCache } from "../../src/cache/admin";
import { countEntries, getEntry, upsertEntities } from "../../src/cache/resource-cache";
import { getSyncState } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { LOCK_STALE_MS, tryAcquireLock } from "../../src/refresh/lock";
import { createRepositories } from "../../src/repository";
import { BASE, makeEntities, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

const hang = () => new Promise<never>(() => {});

afterEach(() => {
  vi.restoreAllMocks();
});
beforeEach(async () => {
  await resetCache();
});

function setup() {
  const clock = fakeClock();
  const db = testDb();
  const deps = { db, client: testClient(clock.now), now: clock.now, pageBudgetFloorMs: 0 };
  return { clock, db, deps, repos: createRepositories(deps) };
}

const clientsKey = `GET ${BASE}/v1/clients`;
const clientRows = () =>
  makeEntities(5, (i) => ({
    name: ["弘前大学ＣＯＩ研究推進機構", "株式会社A", "ClientB", "OtherClientC", "ｶﾌﾞｼｷ"][i],
    name_disp: `略称${i + 1}`,
  }));

describe("cache-first リソース(clients)", () => {
  it("キャッシュが空で refresh=none: ゼロフィルタは API を呼ばず空を返し、cache に『未取得』を載せる", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({});
    const r = await repos.clients.list({}, { refresh: "none" });
    expect(r.items).toEqual([]);
    expect(r.source).toBe("cache");
    expect(r.cache).toEqual([{ resource: "clients" }]);
    expect(api.calls).toHaveLength(0);
  });

  it("キャッシュが空(未同期)でキャッシュ側で絞れるフィルタ: API にフォールバックし、cache は載せない", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({
      [clientsKey]: pagedHandler(() => [{ id: 9, name: "API から" }]),
    });
    const r = await repos.clients.list({ nameCont: "api" });
    expect(r.items).toEqual([{ id: 9, name: "API から" }]);
    expect(r.source).toBe("api");
    expect(r.cache).toEqual([]);
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("name_cont")).toBe("api");
  });

  it("refresh=full のあと、ゼロフィルタはキャッシュ全件、cache に cached_at / full_refreshed_at / refresh_state", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    const r = await repos.clients.list({}, { refresh: "full" });
    expect(r.items).toHaveLength(5);
    expect(r.source).toBe("cache");
    expect(r.cache).toEqual([
      {
        resource: "clients",
        cached_at: "2026-11-05T00:00:00Z",
        full_refreshed_at: "2026-11-05T00:00:00Z",
        refresh_state: "complete",
      },
    ]);
    expect(api.calls).toHaveLength(1);

    // refresh=none で読み直すと、refresh_state は載らない(今回は更新していない)。
    const again = await repos.clients.list({}, { refresh: "none" });
    expect(again.items).toHaveLength(5);
    expect(again.cache[0]?.refresh_state).toBeUndefined();
    expect(again.cache[0]?.cached_at).toBe("2026-11-05T00:00:00Z");
    expect(api.calls).toHaveLength(1);
  });

  it("同期済みなら、キャッシュ側で絞れるフィルタは API を呼ばずに fold で絞る", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    const r = await repos.clients.list({ nameCont: " ｃｏｉ " });
    expect(r.items.map((e) => e.id)).toEqual([1]);
    expect(r.source).toBe("cache");
    expect((await repos.clients.list({ nameCont: "カブシキ" })).items.map((e) => e.id)).toEqual([
      5,
    ]);
    expect(api.calls).toHaveLength(1);
  });

  it("サーバー専用の項目(tags / perPage / updatedAtGteq など)があれば、同期済みでも API を直接呼ぶ", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({
      [clientsKey]: pagedHandler(() => [{ id: 1, name: "live" }]),
    });
    await repos.clients.list({}, { refresh: "full" });
    const r = await repos.clients.list({ nameCont: "x", tags: ["vip"] });
    expect(r.source).toBe("api");
    expect(r.items).toEqual([{ id: 1, name: "live" }]);
    expect(new URL(api.calls.at(-1)?.url ?? "").searchParams.getAll("tags[]")).toEqual(["vip"]);
  });

  it("サーバー専用の項目と refresh=full を同時に指定したら、refresh を走らせず API だけを呼ぶ(M04 指摘)", async () => {
    const { repos, db } = setup();
    const api = mockOutboundFetch({
      [clientsKey]: pagedHandler(() => [{ id: 1, name: "live" }]),
    });
    // 別の呼び出しが refresh 中でも refresh_in_progress にならない(refresh を試みない)。
    await tryAcquireLock(db, "clients");
    for (const refresh of ["full", "delta"] as const) {
      const r = await repos.clients.list({ tags: ["vip"] }, { refresh });
      expect(r.source).toBe("api");
      expect(r.cache).toEqual([]);
    }
    // 通信は API の絞り込み 2 回だけ(refresh の分の通信がない)。
    expect(api.calls).toHaveLength(2);
    for (const c of api.calls) {
      expect(new URL(c.url).searchParams.getAll("tags[]")).toEqual(["vip"]);
    }
    expect((await getSyncState(db, "clients"))?.last_synced_at).toBeNull();
  });

  it("キャッシュで絞れる項目だけなら、refresh 指定はこれまでどおり効く", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    const r = await repos.clients.list({ nameCont: "client" }, { refresh: "full" });
    expect(r.source).toBe("cache");
    expect(r.cache[0]).toMatchObject({ refresh_state: "complete" });
    expect(api.calls).toHaveLength(1);
  });

  it("limit で切ったときだけ totalBeforeLimit を返す", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    const cut = await repos.clients.list({}, { limit: 2 });
    expect(cut.items).toHaveLength(2);
    expect(cut.totalBeforeLimit).toBe(5);
    const exact = await repos.clients.list({}, { limit: 5 });
    expect(exact.totalBeforeLimit).toBeUndefined();
    const none = await repos.clients.list({}, {});
    expect(none.totalBeforeLimit).toBeUndefined();
  });

  it("絞り込みのないキャッシュ読みは SQL の LIMIT で必要な件数だけ読む(件数の先の行は解析しない)。totalBeforeLimit は件数の照会で出す", async () => {
    const { repos, db } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    // 並びの最後(updated_at が最大)に、JSON として解析できない行を足す。全行を解析する実装ならここで例外になる。
    await db
      .prepare(
        "INSERT INTO resource_cache (resource_name, entity_id, payload_json, updated_at, fetched_at, generation) VALUES ('clients', 9999, '{broken', '9999-01-01T00:00:00Z', '2026-11-05T00:00:00Z', 1)",
      )
      .run();
    const cut = await repos.clients.list({}, { limit: 2 });
    expect(cut.items.map((c) => (c as { id: number }).id)).toEqual([1, 2]);
    expect(cut.totalBeforeLimit).toBe(6);
    const all = await repos.clients.list({}, { limit: 6 }).catch((e: unknown) => e);
    expect(all).toBeInstanceOf(SyntaxError); // 解析が必要な範囲に入れば、従来どおり壊れた行は検出される
  });

  it("limit はフィルタのあとに適用する", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    expect((await repos.clients.list({}, { limit: 2 })).items).toHaveLength(2);
    expect((await repos.clients.list({ nameCont: "client" }, { limit: 1 })).items).toHaveLength(1);
    expect((await repos.clients.list({ nameCont: "client" })).items).toHaveLength(2);
  });

  it("search は list と同じ", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    expect((await repos.clients.search({ nameCont: "株式" })).items.map((e) => e.id)).toEqual([2]);
  });

  it("delta が失敗したら古いキャッシュを返し、cache に failed とエラーコード(メッセージは載せない)", async () => {
    const { repos, clock } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    vi.restoreAllMocks();
    clock.advance(1000);
    mockOutboundFetch({ [clientsKey]: () => jsonResponse({ message: "secret detail" }, 500) });
    const r = await repos.clients.list({}, { refresh: "delta" });
    expect(r.items).toHaveLength(5);
    expect(r.cache[0]).toMatchObject({ refresh_state: "failed", refresh_error_code: "api_error" });
    expect(JSON.stringify(r.cache)).not.toContain("secret detail");
  });

  it("full が失敗したら例外が伝わる(Go の ForceRefresh と同じ)", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [clientsKey]: () => jsonResponse({ message: "x" }, 401) });
    await expect(repos.clients.list({}, { refresh: "full" })).rejects.toMatchObject({
      code: "authentication_error",
    });
  });

  it("別の refresh がロックを保持していたら、待たずに RefreshInProgressError", async () => {
    const { repos, db, clock } = setup();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "other" });
    const api = mockOutboundFetch({});
    for (const refresh of ["delta", "full"] as const) {
      const e = await repos.clients.list({}, { refresh }).catch((x) => x);
      expect(e).toBeInstanceOf(RefreshInProgressError);
      expect(e.retryAfterSeconds).toBe(30);
    }
    expect(api.calls).toHaveLength(0);
    // refresh=none の参照はロックの影響を受けない。
    await expect(repos.clients.list({}, { refresh: "none" })).resolves.toMatchObject({ items: [] });
  });

  it("full が締め切りで partial になったら、ここまでのキャッシュを返し refresh_state=partial。続きは次の呼び出しで", async () => {
    const { repos, clock } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [clientsKey]: pagedHandler(
        () => data,
        () => {
          clock.advance(9_000);
          return undefined;
        },
      ),
    });
    const r = await repos.clients.list({}, { refresh: "full" }, { deadline: clock.now() + 10_000 });
    expect(r.items).toHaveLength(100);
    expect(r.cache[0]).toMatchObject({ resource: "clients", refresh_state: "partial" });
    expect(r.cache[0]?.full_refreshed_at).toBeUndefined();

    // 次の refresh=none の参照でも、未完了であることが分かる。
    const again = await repos.clients.list({}, { refresh: "none" });
    expect(again.cache[0]?.refresh_state).toBe("partial");
  });

  it("非ゼロ・キャッシュ可能なフィルタ + refresh=full が partial で未同期なら API にフォールバックし、cache に partial を載せる", async () => {
    const { repos, clock } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [clientsKey]: pagedHandler(
        () => data,
        (_page, q) => {
          // 絞り込み付きの API フォールバックでは時間を進めない(refresh のページ取得だけが遅い)。
          if (!q.has("name_cont")) clock.advance(9_000);
          return undefined;
        },
      ),
    });
    const r = await repos.clients.list(
      { nameCont: "顧客1" },
      { refresh: "full" },
      { deadline: clock.now() + 10_000 },
    );
    expect(r.source).toBe("api");
    expect(r.cache).toHaveLength(1);
    expect(r.cache[0]).toMatchObject({ resource: "clients", refresh_state: "partial" });
  });

  it("refresh=delta が失敗し、キャッシュに壊れた行があって API にフォールバックした場合も、cache に failed とエラーコードを載せる", async () => {
    const { repos, db, clock } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    vi.restoreAllMocks();
    await db
      .prepare(
        "INSERT INTO resource_cache (resource_name, entity_id, payload_json, updated_at, fetched_at, generation) VALUES ('clients', 9999, '{broken', '9999-01-01T00:00:00Z', '2026-11-05T00:00:00Z', 1)",
      )
      .run();
    clock.advance(1000);
    mockOutboundFetch({
      [clientsKey]: (call) =>
        new URL(call.url).searchParams.has("name_cont")
          ? jsonResponse([{ id: 9, name: "API から" }])
          : jsonResponse({ message: "secret detail" }, 500),
    });
    const r = await repos.clients.list({ nameCont: "api" }, { refresh: "delta" });
    expect(r.source).toBe("api");
    expect(r.items).toEqual([{ id: 9, name: "API から" }]);
    expect(r.cache[0]).toMatchObject({
      resource: "clients",
      refresh_state: "failed",
      refresh_error_code: "api_error",
    });
    expect(JSON.stringify(r.cache)).not.toContain("secret detail");
  });

  it("get: refresh のあとキャッシュミスで API から取った場合、cache に refresh の結果を載せる", async () => {
    const { repos } = setup();
    mockOutboundFetch({
      [clientsKey]: pagedHandler(clientRows),
      [`GET ${BASE}/v1/clients/*`]: () =>
        jsonResponse({ id: 77, name: "単体取得", updated_at: "2026-02-01T00:00:00.000+09:00" }),
    });
    const r = await repos.clients.get(77, { refresh: "full" });
    expect(r.source).toBe("api");
    expect(r.cache).toHaveLength(1);
    expect(r.cache[0]).toMatchObject({ resource: "clients", refresh_state: "complete" });
  });

  it("get: refresh=delta が失敗し、キャッシュミスで API から取った場合、cache に failed とエラーコード(メッセージは載せない)", async () => {
    const { repos, clock } = setup();
    mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    await repos.clients.list({}, { refresh: "full" });
    vi.restoreAllMocks();
    clock.advance(1000);
    mockOutboundFetch({
      [clientsKey]: () => jsonResponse({ message: "secret detail" }, 500),
      [`GET ${BASE}/v1/clients/*`]: () =>
        jsonResponse({ id: 77, name: "単体取得", updated_at: "2026-02-01T00:00:00.000+09:00" }),
    });
    const r = await repos.clients.get(77, { refresh: "delta" });
    expect(r.source).toBe("api");
    expect(r.cache).toHaveLength(1);
    expect(r.cache[0]).toMatchObject({
      resource: "clients",
      refresh_state: "failed",
      refresh_error_code: "api_error",
    });
    expect(JSON.stringify(r.cache)).not.toContain("secret detail");
  });

  it("get: refresh=full が partial で、キャッシュミスで API から取った場合、cache に partial を載せる", async () => {
    const { repos, clock } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [clientsKey]: pagedHandler(
        () => data,
        () => {
          clock.advance(9_000);
          return undefined;
        },
      ),
      [`GET ${BASE}/v1/clients/*`]: () =>
        jsonResponse({ id: 9999, name: "単体取得", updated_at: "2026-02-01T00:00:00.000+09:00" }),
    });
    const r = await repos.clients.get(
      9999,
      { refresh: "full" },
      { deadline: clock.now() + 10_000 },
    );
    expect(r.source).toBe("api");
    expect(r.cache).toHaveLength(1);
    expect(r.cache[0]).toMatchObject({ resource: "clients", refresh_state: "partial" });
  });

  it("list: full が partial で未同期でも、refresh=none(既定)で API にフォールバックしたときは cache を載せない", async () => {
    const { repos, clock } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [clientsKey]: pagedHandler(
        () => data,
        (_page, q) => {
          if (!q.has("name_cont")) clock.advance(9_000);
          return undefined;
        },
      ),
    });
    const first = await repos.clients.list(
      {},
      { refresh: "full" },
      { deadline: clock.now() + 10_000 },
    );
    // 前提: full が途中で止まり、未同期のまま。
    expect(first.cache[0]).toMatchObject({ resource: "clients", refresh_state: "partial" });
    const r = await repos.clients.list({ nameCont: "顧客1" });
    expect(r.source).toBe("api");
    expect(r.cache).toEqual([]);
  });

  it("get: キャッシュにあれば API を呼ばない / なければ API から取って保存し、次はキャッシュから返す", async () => {
    const { repos, db } = setup();
    const api = mockOutboundFetch({
      [clientsKey]: pagedHandler(clientRows),
      [`GET ${BASE}/v1/clients/*`]: (call) =>
        jsonResponse({
          id: Number(call.url.split("/").pop()),
          name: "単体取得",
          updated_at: "2026-02-01T00:00:00.000+09:00",
        }),
    });
    const miss = await repos.clients.get(77);
    expect(miss).toMatchObject({ source: "api", item: { id: 77, name: "単体取得" }, cache: [] });
    expect(await getEntry(db, "clients", 77)).not.toBeNull();
    const hit = await repos.clients.get(77);
    expect(hit.source).toBe("cache");
    expect(api.callsTo(`GET ${BASE}/v1/clients/77`)).toHaveLength(1);
  });

  it("get: 存在しなければ not_found が伝わる", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [`GET ${BASE}/v1/clients/*`]: () => jsonResponse({ message: "x" }, 404) });
    await expect(repos.clients.get(5)).rejects.toMatchObject({ code: "not_found" });
  });

  it("get: refresh=full なら先に refresh してから読む", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({ [clientsKey]: pagedHandler(clientRows) });
    const r = await repos.clients.get(3, { refresh: "full" });
    expect(r).toMatchObject({ source: "cache", item: { id: 3 } });
    expect(r.cache[0]).toMatchObject({ refresh_state: "complete" });
    expect(api.calls).toHaveLength(1);
  });

  it("id が不正なら argument_error", async () => {
    const { repos } = setup();
    mockOutboundFetch({});
    await expect(repos.clients.get(0)).rejects.toMatchObject({ code: "argument_error" });
  });
});

describe("nonzero-api リソース(client_branches)", () => {
  const key = `GET ${BASE}/v1/client_branches`;

  it("非ゼロのフィルタは、同期済みでも refresh 指定があってもキャッシュを経由せず API を直接呼ぶ(Go と同じ)", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({ [key]: pagedHandler(() => [{ id: 1, name: "live" }]) });
    await repos.client_branches.list({}, { refresh: "full" });
    const calls = api.calls.length;
    const r = await repos.client_branches.list({ nameCont: "a" }, { refresh: "delta" });
    expect(r.source).toBe("api");
    expect(r.cache).toEqual([]);
    expect(api.calls.length).toBe(calls + 1);
  });

  it("ゼロフィルタはキャッシュ。空なら空(API を呼ばない)", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({});
    const r = await repos.client_branches.list({});
    expect(r).toMatchObject({
      items: [],
      source: "cache",
      cache: [{ resource: "client_branches" }],
    });
    expect(api.calls).toHaveLength(0);
  });

  it("get は cache-first リソースと同じく、キャッシュ → API", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/client_branches/*`]: () => jsonResponse({ id: 4, name: "b" }),
    });
    expect((await repos.client_branches.get(4)).source).toBe("api");
    expect((await repos.client_branches.get(4)).source).toBe("cache");
    expect(api.calls).toHaveLength(1);
  });
});

describe("document リソース(estimates など。get のみ)", () => {
  const key = `GET ${BASE}/v1/documents/estimates/*`;
  const doc = (id: number, total = "100") => jsonResponse({ id, total, details: [] });

  it("キャッシュにあれば返し(cached_at は取得時刻)、なければ API → 保存", async () => {
    const { repos, db, clock } = setup();
    const api = mockOutboundFetch({ [key]: (c) => doc(Number(c.url.split("/").pop())) });
    const miss = await repos.estimates.get(10);
    expect(miss).toMatchObject({ source: "api", item: { id: 10 }, cache: [] });
    expect(await getEntry(db, "estimates", 10)).not.toBeNull();
    clock.advance(5_000);
    const hit = await repos.estimates.get(10);
    expect(hit.source).toBe("cache");
    expect(hit.cache).toEqual([{ resource: "estimates", cached_at: "2026-11-05T00:00:00Z" }]);
    expect(api.calls).toHaveLength(1);
  });

  it("refresh=full はキャッシュを使わず取り直して上書きする。delta は何もしない(キャッシュを使う)", async () => {
    const { repos, clock } = setup();
    let total = "100";
    const api = mockOutboundFetch({ [key]: () => doc(10, total) });
    await repos.estimates.get(10);
    total = "200";
    expect((await repos.estimates.get(10, { refresh: "delta" })).item).toMatchObject({
      total: "100",
    });
    expect(api.calls).toHaveLength(1);
    clock.advance(1_000); // fetched_at は秒精度で、同じ秒の行は上書きしない(新しい行を守る)ので、時計を進める
    const fresh = await repos.estimates.get(10, { refresh: "full" });
    expect(fresh).toMatchObject({ source: "api", item: { total: "200" } });
    expect((await repos.estimates.get(10)).item).toMatchObject({ total: "200" });
  });

  it("list / refresh は持たない", () => {
    const { repos } = setup();
    expect("list" in repos.estimates).toBe(false);
    expect("refresh" in repos.estimates).toBe(false);
    expect(Object.keys(repos)).toHaveLength(22);
  });
});

describe("projects の response_group 付き get", () => {
  it("キャッシュを経由せず API を直接呼ぶ(埋め込みを再構築できないため)", async () => {
    const { repos, db } = setup();
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/projects/*`]: () => jsonResponse({ id: 3, name: "p", estimate: { id: 1 } }),
    });
    const r = await repos.projects.getWithGroup(3, "estimate");
    expect(r).toMatchObject({ source: "api", item: { id: 3, estimate: { id: 1 } }, cache: [] });
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("response_group")).toBe("estimate");
    expect(await getEntry(db, "projects", 3)).toBeNull();
  });
});

describe("締め切りとキャンセルは呼び出し元の CallContext に従う", () => {
  it("signal が abort 済みなら、API を呼ぶ get は api_error(retryable)で失敗する", async () => {
    const { repos } = setup();
    mockOutboundFetch({ [`GET ${BASE}/v1/clients/*`]: hang });
    const ac = new AbortController();
    ac.abort();
    await expect(repos.clients.get(1, {}, { signal: ac.signal })).rejects.toMatchObject({
      code: "api_error",
      retryable: true,
    });
  });

  it("refresh はロックを解放する(失敗しても)", async () => {
    const { repos, db } = setup();
    mockOutboundFetch({ [clientsKey]: () => jsonResponse({}, 500) });
    await repos.clients.list({}, { refresh: "full" }).catch(() => undefined);
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
  });
});

describe("単体取得のキャッシュ書き込み(世代・ロック・ID の確認)", () => {
  const itemKey = `GET ${BASE}/v1/clients/*`;
  const idOf = (url: string) => Number(url.split("/").pop());
  const gen = async (db: D1Database, id: number) => (await getEntry(db, "clients", id))?.generation;

  it("世代は INSERT の中で sync_state から読む(読んでから書くまでの間に変わっても、書く時点の世代になる)", async () => {
    const { repos, db } = setup();
    await db
      .prepare("INSERT INTO sync_state (resource_name, full_generation) VALUES ('clients', 4)")
      .run();
    mockOutboundFetch({
      [itemKey]: async (c) => {
        // API を待っている間に世代が進んだ(別の呼び出しの full)。
        await db
          .prepare("UPDATE sync_state SET full_generation = 7 WHERE resource_name = 'clients'")
          .run();
        return jsonResponse({ id: idOf(c.url), name: "x" });
      },
    });
    const r = await repos.clients.get(10);
    expect(r.source).toBe("api");
    expect(await gen(db, 10)).toBe(7);
  });

  it("sync_state の行がなければ世代 0", async () => {
    const { repos, db } = setup();
    mockOutboundFetch({ [itemKey]: (c) => jsonResponse({ id: idOf(c.url), name: "x" }) });
    await repos.clients.get(10);
    expect(await gen(db, 10)).toBe(0);
  });

  it("cache_clear の直後に戻った行は世代 0 になり、次の full で消える(古い大きい世代で残らない)", async () => {
    const { repos, db, clock } = setup();
    const rows = () => makeEntities(5);
    mockOutboundFetch({
      [clientsKey]: pagedHandler(rows),
      [itemKey]: async (c) => {
        // API を待っている間に cache_clear が走る。
        await clearCache(db, { now: clock.now, resources: ["clients"] });
        return jsonResponse({ id: idOf(c.url), name: "stale" });
      },
    });
    await repos.clients.refresh("full");
    await repos.clients.refresh("full");
    expect(await gen(db, 1)).toBe(2);

    await repos.clients.get(77);
    expect(await gen(db, 77)).toBe(0); // 古い世代 2 を引きずらない
    expect(await getSyncState(db, "clients")).toBeNull();

    clock.advance(1_000);
    await repos.clients.refresh("full"); // 世代 1 に数え直し
    expect((await repos.clients.refresh("full")).state).toBe("complete");
    expect(await getEntry(db, "clients", 77)).toBeNull();
    expect(await countEntries(db, "clients")).toBe(5);
  });

  it("その resource のロックが取られている間は書かない。応答は返す。解放後は書く", async () => {
    const { repos, db, clock } = setup();
    mockOutboundFetch({ [itemKey]: (c) => jsonResponse({ id: idOf(c.url), name: "x" }) });
    const lock = await tryAcquireLock(db, "clients", { now: clock.now, owner: "other" });
    const r = await repos.clients.get(10);
    expect(r).toMatchObject({ source: "api", item: { id: 10 } });
    expect(await getEntry(db, "clients", 10)).toBeNull();
    // 他の resource のロックは関係ない。
    expect(lock.resource).toBe("clients");

    await db.prepare("UPDATE sync_state SET refresh_started_at = NULL, refresh_owner = NULL").run();
    await repos.clients.get(10);
    expect(await getEntry(db, "clients", 10)).not.toBeNull();
  });

  it("stale になったロック(既存のロックの定義と同じ LOCK_STALE_MS 超)は書き込みを止めない。ちょうど境界までは止める", async () => {
    const { repos, db, clock } = setup();
    mockOutboundFetch({ [itemKey]: (c) => jsonResponse({ id: idOf(c.url), name: "x" }) });
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "other" });
    clock.advance(LOCK_STALE_MS);
    await repos.clients.get(10);
    expect(await getEntry(db, "clients", 10)).toBeNull(); // 取得側の奪取条件(started < now - STALE)を満たさない = まだ保持中
    clock.advance(1_000);
    await repos.clients.get(10);
    expect(await getEntry(db, "clients", 10)).not.toBeNull();
  });

  it("別の resource のロックは書き込みを止めない", async () => {
    const { repos, db, clock } = setup();
    mockOutboundFetch({ [itemKey]: (c) => jsonResponse({ id: idOf(c.url), name: "x" }) });
    await tryAcquireLock(db, "vendors", { now: clock.now, owner: "other" });
    await repos.clients.get(10);
    expect(await getEntry(db, "clients", 10)).not.toBeNull();
  });

  it.each([
    ["別の ID", { id: 11, name: "x" }],
    ["id が文字列", { id: "10", name: "x" }],
    ["id がない", { name: "x" }],
    ["id が小数", { id: 10.5, name: "x" }],
    ["id が null", { id: null, name: "x" }],
  ])(
    "応答の item.id が要求した ID と違う(%s)ときはキャッシュしないが、応答は返す",
    async (_n, body) => {
      const { repos, db } = setup();
      mockOutboundFetch({ [itemKey]: () => jsonResponse(body) });
      const r = await repos.clients.get(10);
      expect(r).toMatchObject({ source: "api", item: body });
      expect(await countEntries(db, "clients")).toBe(0);
    },
  );

  it("書類の取得も同じ: ID が違えばキャッシュしない / 一致すれば保存する", async () => {
    const { repos, db } = setup();
    const key = `GET ${BASE}/v1/documents/estimates/*`;
    mockOutboundFetch({
      [key]: (c) => jsonResponse({ id: idOf(c.url) === 10 ? 99 : idOf(c.url) }),
    });
    expect((await repos.estimates.get(10)).item).toEqual({ id: 99 });
    expect(await countEntries(db, "estimates")).toBe(0);
    await repos.estimates.get(11);
    expect(await getEntry(db, "estimates", 11)).not.toBeNull();
  });

  it("書類の取得も、その resource のロック(同名の sync_state)があれば書かない", async () => {
    const { repos, db, clock } = setup();
    const key = `GET ${BASE}/v1/documents/estimates/*`;
    mockOutboundFetch({ [key]: (c) => jsonResponse({ id: idOf(c.url) }) });
    await tryAcquireLock(db, "estimates", { now: clock.now, owner: "other" });
    await repos.estimates.get(11);
    expect(await countEntries(db, "estimates")).toBe(0);
  });
  it("API を待っている間に別の書き込みが同じ ID の新しい行を書いたら、取得結果で上書きしない(list 型)", async () => {
    const { repos, db, clock } = setup();
    mockOutboundFetch({
      [itemKey]: async (c) => {
        clock.advance(2_000);
        await upsertEntities(db, "clients", [{ id: idOf(c.url), name: "fresh" }], {
          now: clock.now(),
        });
        return jsonResponse({ id: idOf(c.url), name: "stale" });
      },
    });
    const r = await repos.clients.get(10);
    expect(r).toMatchObject({ source: "api", item: { name: "stale" } });
    expect((await getEntry<{ name: string }>(db, "clients", 10))?.entity.name).toBe("fresh");
  });

  it("書類も同じ: 取得中に書かれた新しい行を残す", async () => {
    const { repos, db, clock } = setup();
    const key = `GET ${BASE}/v1/documents/estimates/*`;
    mockOutboundFetch({
      [key]: async (c) => {
        clock.advance(2_000);
        await upsertEntities(db, "estimates", [{ id: idOf(c.url), name: "fresh" }], {
          now: clock.now(),
        });
        return jsonResponse({ id: idOf(c.url), name: "stale" });
      },
    });
    await repos.estimates.get(11);
    expect((await getEntry<{ name: string }>(db, "estimates", 11))?.entity.name).toBe("fresh");
  });

  it("書類の refresh=full は、古い既存の行を取り直した結果で更新する", async () => {
    const { repos, db, clock } = setup();
    const key = `GET ${BASE}/v1/documents/estimates/*`;
    await upsertEntities(db, "estimates", [{ id: 11, name: "old" }], { now: clock.now() });
    clock.advance(5_000);
    mockOutboundFetch({ [key]: (c) => jsonResponse({ id: idOf(c.url), name: "new" }) });
    await repos.estimates.get(11, { refresh: "full" });
    expect((await getEntry<{ name: string }>(db, "estimates", 11))?.entity.name).toBe("new");
  });
});
