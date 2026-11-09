/**
 * issue #27: users のキャッシュ専用キーと、purchase_orders の既定 response_group(medium)の repository での振る舞い。
 * 4 リソースの実データの形のフィクスチャ(test/helpers/entities.ts)を使う。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createRepositories } from "../../src/repository";
import { BASE, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { purchaseOrderEntity, userEntity } from "../helpers/entities";
import { mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

function setup() {
  const clock = fakeClock();
  const deps = {
    db: testDb(),
    client: testClient(clock.now),
    now: clock.now,
    pageBudgetFloorMs: 0,
  };
  return createRepositories(deps);
}

const usersKey = `GET ${BASE}/v1/users`;
const poKey = `GET ${BASE}/v1/expenditures`;

const userRows = () => [
  userEntity({ id: 1, last_name: "山田", first_name: "太郎", email: "taro@example.com" }),
  userEntity({ id: 2, last_name: "鈴木", first_name: "花子", email: "hanako@example.com" }),
  userEntity({ id: 3, last_name: "山田", first_name: "次郎", email: "jiro@corp.example" }),
];

describe("users: name_cont / email_cont はキャッシュ専用のキー", () => {
  it("未同期: 絞り込みを付けずに API から全ページ取り、match で絞って返す(API に name_cont / email_cont を送らない)", async () => {
    const repos = setup();
    // 3 ページにまたがる 250 件 + 該当 2 件
    const many = Array.from({ length: 250 }, (_, i) =>
      userEntity({ id: 100 + i, last_name: `姓${i}`, first_name: `名${i}`, email: `u${i}@x.test` }),
    );
    const api = mockOutboundFetch({ [usersKey]: pagedHandler(() => [...userRows(), ...many]) });
    const r = await repos.users.list({ nameCont: "山田", emailCont: "example" });
    expect(r.source).toBe("api");
    expect(r.items.map((u) => u.id)).toEqual([1, 3]);
    expect(api.calls.length).toBeGreaterThan(1); // 全ページを取った
    for (const call of api.calls) {
      const q = new URL(call.url).searchParams;
      expect([...q.keys()].filter((k) => k !== "page" && k !== "per_page")).toEqual([]);
    }
  });

  it("未同期: limit は match のあとに適用する", async () => {
    const repos = setup();
    mockOutboundFetch({ [usersKey]: pagedHandler(userRows) });
    const r = await repos.users.list({ nameCont: "山田" }, { limit: 1 });
    expect(r.items.map((u) => u.id)).toEqual([1]);
    expect(r.totalBeforeLimit).toBe(2);
  });

  it("同期済み: キャッシュで絞り(姓・名・姓名)、API は呼ばない", async () => {
    const repos = setup();
    const api = mockOutboundFetch({ [usersKey]: pagedHandler(userRows) });
    await repos.users.list({}, { refresh: "full" });
    const callsAfterSync = api.calls.length;
    const byFull = await repos.users.list({ nameCont: "山田 太郎" });
    expect(byFull.source).toBe("cache");
    expect(byFull.items.map((u) => u.id)).toEqual([1]);
    expect((await repos.users.list({ nameCont: "花子" })).items.map((u) => u.id)).toEqual([2]);
    expect((await repos.users.list({ emailCont: "CORP" })).items.map((u) => u.id)).toEqual([3]);
    expect(api.calls).toHaveLength(callsAfterSync);
  });

  it("名前の一部がキャッシュ上の姓にも名にも無ければ 0 件(誤って全件を返さない)", async () => {
    const repos = setup();
    mockOutboundFetch({ [usersKey]: pagedHandler(userRows) });
    await repos.users.list({}, { refresh: "full" });
    expect((await repos.users.list({ nameCont: "存在しない" })).items).toEqual([]);
  });
});

describe("purchase_orders: 一覧の response_group は未指定なら medium", () => {
  it("refresh(full / delta)が medium を送る", async () => {
    const repos = setup();
    const api = mockOutboundFetch({ [poKey]: pagedHandler(() => [purchaseOrderEntity()]) });
    await repos.purchase_orders.list({}, { refresh: "full" });
    await repos.purchase_orders.list({}, { refresh: "delta" });
    expect(api.calls.length).toBeGreaterThanOrEqual(2);
    for (const call of api.calls) {
      expect(new URL(call.url).searchParams.get("response_group")).toBe("medium");
    }
  });

  it("API へのフォールバックも medium。明示した response_group はそのまま送る", async () => {
    const repos = setup();
    const api = mockOutboundFetch({ [poKey]: pagedHandler(() => [purchaseOrderEntity()]) });
    await repos.purchase_orders.list({ projectIdEq: 100 });
    await repos.purchase_orders.list({ projectIdEq: 100, responseGroup: "small" });
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("response_group"))).toEqual([
      "medium",
      "small",
    ]);
  });

  it("project_id_eq はキャッシュでは絞らない: 同期済みでも API を呼ぶ", async () => {
    const repos = setup();
    const api = mockOutboundFetch({ [poKey]: pagedHandler(() => [purchaseOrderEntity()]) });
    await repos.purchase_orders.list({}, { refresh: "full" });
    const before = api.calls.length;
    const r = await repos.purchase_orders.list({ projectIdEq: 100 });
    expect(r.source).toBe("api");
    expect(api.calls).toHaveLength(before + 1);
    expect(new URL(api.calls[before]?.url ?? BASE).searchParams.get("project_id_eq")).toBe("100");
  });
});
