/**
 * issue #37: キャッシュに書く get は response_group=small を明示して送り、行の項目数を経路によらず揃える。
 * 項目が要るときは get_* の response_group(API を直接呼び、キャッシュには書かない)。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getEntry } from "../../src/cache/resource-cache";
import { BASE, callApiTool, describeApiCall } from "../helpers/api-tool";
import { resetCache, testDb } from "../helpers/d1";
import { jsonResponse } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const CASES = [
  {
    tool: "get_project",
    resource: "projects",
    path: "/v1/projects",
    cache: "small",
    group: "estimate",
  },
  {
    tool: "get_purchase_order",
    resource: "purchase_orders",
    path: "/v1/expenditures",
    cache: "medium",
    group: "large",
  },
] as const;

// purchase_orders の一覧の既定は medium(案件 project を含む)。refresh の行と揃えるため、get もキャッシュ用には medium を送る。
describe.each(CASES)("$tool", ({ tool, resource, path, cache, group }) => {
  const key = (id: number) => ({
    [`GET ${BASE}${path}/${id}`]: () => jsonResponse({ id, name: `n${id}` }),
  });

  it("キャッシュに書く get は response_group を明示して送る(projects は small、purchase_orders は一覧の refresh と同じ medium)。2 回目はキャッシュで API を呼ばない", async () => {
    const a = await callApiTool(tool, { id: 7 }, key(7));
    expect(a.calls.map(describeApiCall)).toEqual([`GET ${path}/7?response_group=${cache}`]);
    expect(await getEntry(testDb(), resource, 7)).not.toBeNull();
    const b = await callApiTool(tool, { id: 7 }, key(7));
    expect(b.calls).toHaveLength(0);
    expect(b.json.item).toEqual({ id: 7, name: "n7" });
  });

  it("response_group を指定すると、その値で API を直接呼び、キャッシュには書かない(キャッシュ済みでも API)", async () => {
    const r = await callApiTool(tool, { id: 8, response_group: group }, key(8));
    expect(r.isError).toBe(false);
    expect(r.calls.map(describeApiCall)).toEqual([`GET ${path}/8?response_group=${group}`]);
    expect(r.json.cache).toEqual([]);
    expect(await getEntry(testDb(), resource, 8)).toBeNull();

    // キャッシュに載せたあとでも、指定すれば API を呼び、refresh は無視する
    await callApiTool(tool, { id: 9 }, key(9));
    const again = await callApiTool(
      tool,
      { id: 9, response_group: group, refresh: "full" },
      key(9),
    );
    expect(again.calls.map(describeApiCall)).toEqual([`GET ${path}/9?response_group=${group}`]);
    // 書かれた行はキャッシュ用の項目のまま(上書きされない)
    expect((await getEntry(testDb(), resource, 9))?.entity).toEqual({ id: 9, name: "n9" });
  });
});

describe("get_purchase_order の response_group の列挙値(OpenAPI v1.9.0 の GET /expenditures/{id})", () => {
  const key = { [`GET ${BASE}/v1/expenditures/5`]: () => jsonResponse({ id: 5 }) };

  it.each(["small", "medium", "large", "estimate", "order", "delivery", "invoice", "all"])(
    "%s は受ける",
    async (group) => {
      const r = await callApiTool("get_purchase_order", { id: 5, response_group: group }, key);
      expect(r.isError).toBe(false);
      expect(r.calls.map(describeApiCall)).toEqual([
        `GET /v1/expenditures/5?response_group=${group}`,
      ]);
    },
  );

  it("列挙にない値は argument_error で、API を呼ばない", async () => {
    const r = await callApiTool(
      "get_purchase_order",
      { id: 5, response_group: "project_cost" },
      key,
    );
    expect(r.isError).toBe(true);
    expect(r.calls).toHaveLength(0);
  });
});
