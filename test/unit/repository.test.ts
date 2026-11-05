import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getEntry } from "../../src/cache/resource-cache";
import { getSyncState } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { tryAcquireLock } from "../../src/refresh/lock";
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
    const { repos } = setup();
    let total = "100";
    const api = mockOutboundFetch({ [key]: () => doc(10, total) });
    await repos.estimates.get(10);
    total = "200";
    expect((await repos.estimates.get(10, { refresh: "delta" })).item).toMatchObject({
      total: "100",
    });
    expect(api.calls).toHaveLength(1);
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
