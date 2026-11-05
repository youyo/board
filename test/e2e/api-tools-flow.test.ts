import { env } from "cloudflare:workers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { clearBoardClientCache } from "../../src/board/client";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { BASE, makeEntities, pagedHandler } from "../helpers/board-list";
import { resetCache } from "../helpers/d1";
import { firstText, readRpc } from "../helpers/mcp";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";
import { mcpCall } from "../helpers/worker";

// Worker 全体(Access の JWT 検証 → /mcp → ツール → repository → D1 / 本物の BOARD クライアント)を HTTP で通す。
// BOARD API は outbound fetch のモック。実 BOARD には通信しない。

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;
const SECRETS = ["test-board-api-key", "test-board-api-token"];

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
  vi.useRealTimers();
  vi.restoreAllMocks();
  clearJwksCache();
  clearBoardClientCache();
});

function boardMocks(handlers: Parameters<typeof mockOutboundFetch>[0]) {
  return mockOutboundFetch({ [CERTS]: () => jwksResponse(key), ...handlers });
}
const boardCalls = (m: ReturnType<typeof mockOutboundFetch>) =>
  m.calls.filter((c) => c.url.startsWith(BASE));

async function tool(name: string, args: Record<string, unknown>, id = 1) {
  const res = await readRpc(await mcpCall(jwt, "tools/call", { name, arguments: args }, id));
  const text = firstText(res);
  return { isError: res.result?.isError === true, text, json: JSON.parse(text) };
}

describe("E2E: tools/list", () => {
  it("server_info + api 層 40 個 + find 層 11 個 + cache 系 2 個 = 54 個が出る", async () => {
    boardMocks({});
    const res = await readRpc(await mcpCall(jwt, "tools/list"));
    const names = (res.result?.tools ?? []).map((t) => t.name);
    expect(names).toHaveLength(54);
    expect(names.filter((n) => n.startsWith("list_"))).toHaveLength(18);
    expect(names.filter((n) => n.startsWith("get_"))).toHaveLength(22);
    expect(names).toContain("server_info");
    for (const t of res.result?.tools ?? []) {
      // cache_clear だけが破壊的(cache 系は cache-tools-flow.test.ts)。
      expect(t.annotations?.readOnlyHint, t.name).toBe(t.name !== "cache_clear");
    }
  });
});

describe("E2E: list_clients", () => {
  it("キャッシュが空 → refresh=full で items → refresh=none は API を呼ばずキャッシュから", async () => {
    const m = boardMocks({ [`GET ${BASE}/v1/clients`]: pagedHandler(() => makeEntities(3)) });

    const empty = await tool("list_clients", {});
    expect(empty.isError, empty.text).toBe(false);
    expect(empty.json).toEqual({ items: [], cache: [{ resource: "clients" }] });
    expect(boardCalls(m)).toHaveLength(0);

    const full = await tool("list_clients", { refresh: "full" }, 2);
    expect(full.isError, full.text).toBe(false);
    expect(full.json.items).toHaveLength(3);
    expect(full.json.items[0]).toMatchObject({ id: 1, name: "顧客1" });
    expect(full.json.cache[0]).toMatchObject({ resource: "clients", refresh_state: "complete" });
    expect(boardCalls(m)).toHaveLength(1);

    const none = await tool("list_clients", { refresh: "none" }, 3);
    expect(none.json.items).toHaveLength(3);
    expect(none.json.cache[0].cached_at).toBeTruthy();
    expect(none.json.cache[0].full_refreshed_at).toBeTruthy();
    expect(boardCalls(m)).toHaveLength(1);

    const limited = await tool("list_clients", { limit: 2 }, 4);
    expect(limited.json.items).toHaveLength(2);
    expect(boardCalls(m)).toHaveLength(1);
  });
});

describe("E2E: get_delivery", () => {
  it("API から取り、2 回目はキャッシュから返す", async () => {
    const m = boardMocks({
      [`GET ${BASE}/v1/documents/deliveries/9`]: () => jsonResponse({ id: 9, no: "D-9" }),
    });
    const first = await tool("get_delivery", { id: 9, show_meta: true });
    expect(first.isError, first.text).toBe(false);
    expect(first.json.item).toEqual({ id: 9, no: "D-9" });
    expect(first.json._meta).toBeDefined();
    expect(boardCalls(m)).toHaveLength(1);

    const second = await tool("get_delivery", { id: 9 }, 2);
    expect(second.json.item).toEqual({ id: 9, no: "D-9" });
    expect("_meta" in second.json).toBe(false);
    expect(boardCalls(m)).toHaveLength(1);
  });
});

describe("E2E: list_contacts(非ゼロのフィルタ)", () => {
  it("キャッシュを経由せず BOARD を直接呼び、refresh は無視される", async () => {
    const m = boardMocks({
      [`GET ${BASE}/v1/contacts`]: pagedHandler(() => [{ id: 1, name: "山田", client_id: 7 }]),
    });
    const r = await tool("list_contacts", { client_id: 7, name_cont: "山田", refresh: "full" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json).toEqual({ items: [{ id: 1, name: "山田", client_id: 7 }], cache: [] });
    const calls = boardCalls(m);
    expect(calls).toHaveLength(1);
    const q = new URL(calls[0]?.url ?? "").searchParams;
    expect(q.get("client_id_eq")).toBe("7");
    expect(q.get("name_cont")).toBe("山田");
    const row = await env.DB.prepare("SELECT COUNT(*) AS n FROM resource_cache").first<{
      n: number;
    }>();
    expect(row?.n).toBe(0);
  });
});

describe("E2E: 締め切りとエラー", () => {
  it("締め切りを超えると retryable のエラー(待たずに失敗する)", async () => {
    // Date だけを偽装する(setTimeout は本物)。BOARD の応答の途中で 60 秒進め、30 秒の持ち時間を使い切らせる。
    vi.useFakeTimers({ toFake: ["Date"] });
    const m = boardMocks({
      [`GET ${BASE}/v1/clients`]: () => {
        vi.setSystemTime(Date.now() + 60_000);
        return new Response(JSON.stringify({ message: "busy" }), {
          status: 503,
          headers: { "Retry-After": "1", "Content-Type": "application/json" },
        });
      },
    });
    const r = await tool("list_clients", { name_cont: "x" });
    expect(r.isError).toBe(true);
    expect(r.json.error.retryable).toBe(true);
    expect(r.json.error.code).toBe("api_error");
    expect(r.json.error.message).toContain("before the deadline");
    // 再試行せずに(Retry-After を待たずに)失敗する。
    expect(boardCalls(m)).toHaveLength(1);
  });

  it("refresh が進行中なら refresh_in_progress(retry_after_seconds 付き)", async () => {
    boardMocks({});
    const { tryAcquireLock } = await import("../../src/refresh/lock");
    await tryAcquireLock(env.DB, "clients");
    const r = await tool("list_clients", { refresh: "delta" });
    expect(r.isError).toBe(true);
    expect(r.json.error).toMatchObject({
      code: "refresh_in_progress",
      retryable: true,
      retry_after_seconds: 30,
    });
  });
});

describe("E2E: 秘密情報", () => {
  it("正常・BOARD のエラー・引数エラーのどの応答にも API キー・トークン・JWT が出ない", async () => {
    boardMocks({
      [`GET ${BASE}/v1/clients/1`]: () => jsonResponse({ id: 1, name: "ok" }),
      [`GET ${BASE}/v1/users/2`]: () =>
        jsonResponse(
          { message: `invalid key test-board-api-key / token test-board-api-token` },
          401,
        ),
    });
    const responses = [
      await tool("get_client", { id: 1, show_meta: true }),
      await tool("get_user", { id: 2 }, 2),
      await tool("get_client", { id: 0 }, 3),
      await tool("list_clients", { bogus: true }, 4),
    ];
    expect(responses[1]?.isError).toBe(true);
    for (const r of responses) {
      for (const secret of [...SECRETS, jwt]) expect(r.text).not.toContain(secret);
    }
  });

  it("D1 の全テーブルに秘密が入らない", async () => {
    boardMocks({ [`GET ${BASE}/v1/clients`]: pagedHandler(() => makeEntities(2)) });
    await tool("list_clients", { refresh: "full" });
    for (const table of ["resource_cache", "sync_state", "cache_meta"]) {
      const rows = await env.DB.prepare(`SELECT * FROM ${table}`).all();
      const dump = JSON.stringify(rows.results);
      for (const secret of [...SECRETS, jwt]) expect(dump).not.toContain(secret);
    }
  });
});
