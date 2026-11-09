import { env } from "cloudflare:workers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { clearBoardClientCache } from "../../src/board/client";
import { tryAcquireLock } from "../../src/refresh/lock";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { BASE, makeEntities, pagedHandler } from "../helpers/board-list";
import { resetCache } from "../helpers/d1";
import { firstText, readRpc } from "../helpers/mcp";
import { mockOutboundFetch } from "../helpers/mock-fetch";
import { mcpCall } from "../helpers/worker";

// Worker 全体(Access の JWT 検証 → /mcp → cache 系ツール → D1)を HTTP で通す。
// list_clients で refresh=full して D1 を育て、cache_status で見え、cache_clear で消えることを確かめる。

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;

let key: TestKey;
let jwt: string;

beforeEach(async () => {
  clearJwksCache();
  clearBoardClientCache();
  await resetCache(env.DB);
  key = await createTestKey("kid-1");
  jwt = await signAccessJwt(key);
});
afterEach(() => {
  vi.restoreAllMocks();
  clearJwksCache();
  clearBoardClientCache();
});

async function tool(name: string, args: Record<string, unknown>, id = 1) {
  const res = await readRpc(await mcpCall(jwt, "tools/call", { name, arguments: args }, id));
  const text = firstText(res);
  return { isError: res.result?.isError === true, text, json: JSON.parse(text) };
}

describe("E2E: cache_status / cache_clear", () => {
  it("full refresh の結果が cache_status に出て、cache_clear で消える(逆引きは既定で残る)", async () => {
    const m = mockOutboundFetch({
      [CERTS]: () => jwksResponse(key),
      [`GET ${BASE}/v1/clients`]: pagedHandler(() => makeEntities(3)),
    });

    const before = await tool("cache_status", {});
    expect(before.isError, before.text).toBe(false);
    expect(before.json.resources).toHaveLength(22);
    expect(before.json.resources.every((r: { row_count: number }) => r.row_count === 0)).toBe(true);

    await tool("list_clients", { refresh: "full" }, 2);
    const apiCalls = m.calls.filter((c) => c.url.startsWith(BASE)).length;

    const after = await tool("cache_status", {}, 3);
    const clients = after.json.resources.find(
      (r: { resource: string }) => r.resource === "clients",
    );
    expect(clients).toMatchObject({
      row_count: 3,
      refresh_state: "complete",
      full_prune_skipped: false,
      locked: false,
    });
    expect(clients.cached_at).toBeTruthy();
    expect(clients.full_refreshed_at).toBeTruthy();
    // cache_status は BOARD を呼ばない。
    expect(m.calls.filter((c) => c.url.startsWith(BASE))).toHaveLength(apiCalls);

    const cleared = await tool("cache_clear", { resources: ["clients"] }, 4);
    expect(cleared.json).toEqual({ cleared: ["clients"], document_map_cleared: false });
    const final = await tool("cache_status", {}, 5);
    expect(
      final.json.resources.find((r: { resource: string }) => r.resource === "clients"),
    ).toMatchObject({ row_count: 0, refresh_state: "not_synced" });
  });

  it("書類の単体取得 → cache_status で on_demand と件数。get_purchase_order は response_group=medium でキャッシュ、指定すれば API 直接(issue #37・#38)", async () => {
    const m = mockOutboundFetch({
      [CERTS]: () => jwksResponse(key),
      [`GET ${BASE}/v1/documents/estimates/4`]: () => Response.json({ id: 4 }),
      [`GET ${BASE}/v1/expenditures/6`]: () => Response.json({ id: 6 }),
    });
    expect((await tool("get_estimate", { id: 4 })).isError).toBe(false);
    expect((await tool("get_purchase_order", { id: 6 }, 2)).isError).toBe(false);
    const withGroup = await tool("get_purchase_order", { id: 6, response_group: "large" }, 3);
    expect(withGroup.json.cache).toEqual([]);
    const urls = m.calls.map((c) => c.url).filter((u) => u.includes("/expenditures/"));
    expect(urls).toEqual([
      `${BASE}/v1/expenditures/6?response_group=medium`,
      `${BASE}/v1/expenditures/6?response_group=large`,
    ]);

    const status = await tool("cache_status", {}, 4);
    const by = (n: string) =>
      status.json.resources.find((r: { resource: string }) => r.resource === n);
    expect(by("estimates")).toMatchObject({ refresh_state: "on_demand", row_count: 1 });
    expect(by("orders")).toMatchObject({ refresh_state: "on_demand", row_count: 0 });
    expect(by("purchase_orders").row_count).toBe(1);
    expect(by("users").refresh_state).toBe("not_synced");
  });

  it("refresh 中のリソースがあると cache_clear は refresh_in_progress(retry_after_seconds 30)", async () => {
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    await tryAcquireLock(env.DB, "clients", { owner: "other" });
    const r = await tool("cache_clear", { resources: ["clients"] });
    expect(r.isError).toBe(true);
    expect(r.json.error).toMatchObject({ code: "refresh_in_progress", retry_after_seconds: 30 });
    const status = await tool("cache_status", {}, 2);
    expect(
      status.json.resources.find((x: { resource: string }) => x.resource === "clients").locked,
    ).toBe(true);
  });
});
