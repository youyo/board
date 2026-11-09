import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createRepositories } from "../../src/repository";
import { BASE, makeEntities, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => {
  vi.restoreAllMocks();
});
beforeEach(async () => {
  await resetCache();
});

const clientsKey = `GET ${BASE}/v1/clients`;

function setup(total: number, opts: { totalHeader?: boolean } = { totalHeader: true }) {
  const clock = fakeClock();
  const deps = {
    db: testDb(),
    client: testClient(clock.now),
    now: clock.now,
    pageBudgetFloorMs: 0,
  };
  const api = mockOutboundFetch({
    [clientsKey]: pagedHandler(() => makeEntities(total), undefined, {
      totalHeader: opts.totalHeader,
    }),
  });
  const pages = () =>
    api.calls.map((c) => {
      const q = new URL(c.url).searchParams;
      return { page: Number(q.get("page")), per_page: Number(q.get("per_page")) };
    });
  return { repos: createRepositories(deps), api, pages };
}

// customNoEq はサーバー専用の絞り込み(キャッシュでは扱えない)ので、必ず API 経路になる。
describe("API 経路の limit(issue #33)", () => {
  it("limit=1 で複数ページある: 2 ページ目以降を取りに行かず、limited 用に total を返す", async () => {
    const { repos, pages } = setup(263);
    const r = await repos.clients.list({ customNoEq: "X" }, { limit: 1 });
    expect(r.source).toBe("api");
    expect(r.items).toHaveLength(1);
    expect(r.items[0]?.id).toBe(1);
    expect(pages()).toHaveLength(1);
    expect(pages()[0]?.page).toBe(1);
    expect(pages()[0]?.per_page).toBeLessThanOrEqual(100);
    expect(r.totalBeforeLimit).toBe(263);
    expect(r.truncated).toBe(false);
  });

  it("limit=150 で 263 件: 2 ページで止め、先頭 150 件を返す", async () => {
    const { repos, pages } = setup(263);
    const r = await repos.clients.list({ customNoEq: "X" }, { limit: 150 });
    expect(r.items).toHaveLength(150);
    expect(pages().map((p) => p.page)).toEqual([1, 2]);
    expect(r.totalBeforeLimit).toBe(263);
  });

  it("X-Total-Count が無いとき: limit を超えて取れた件数を totalBeforeLimit にする", async () => {
    const { repos, pages } = setup(263, { totalHeader: false });
    const r = await repos.clients.list({ customNoEq: "X" }, { limit: 1 });
    expect(r.items).toHaveLength(1);
    expect(pages()).toHaveLength(1);
    expect(r.totalBeforeLimit).toBeGreaterThan(1);
  });

  it("limit が総件数以上: 切り詰めは起きず totalBeforeLimit を出さない", async () => {
    const { repos } = setup(3);
    const r = await repos.clients.list({ customNoEq: "X" }, { limit: 3 });
    expect(r.items).toHaveLength(3);
    expect(r.totalBeforeLimit).toBeUndefined();
    const r2 = await repos.clients.list({ customNoEq: "X" }, { limit: 10 });
    expect(r2.items).toHaveLength(3);
    expect(r2.totalBeforeLimit).toBeUndefined();
  });

  it("limit が総件数と同じでヘッダーが無い: limited にしない", async () => {
    const { repos } = setup(2, { totalHeader: false });
    const r = await repos.clients.list({ customNoEq: "X" }, { limit: 2 });
    expect(r.items).toHaveLength(2);
    expect(r.totalBeforeLimit).toBeUndefined();
  });

  it("limit 未指定は従来どおり全ページを取る", async () => {
    const { repos, pages } = setup(263);
    const r = await repos.clients.list({ customNoEq: "X" });
    expect(r.items).toHaveLength(263);
    expect(pages().map((p) => p.page)).toEqual([1, 2, 3]);
    expect(pages().every((p) => p.per_page === 100)).toBe(true);
    expect(r.totalBeforeLimit).toBeUndefined();
  });
});
