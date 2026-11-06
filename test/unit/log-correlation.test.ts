import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { actorOf } from "../../src/app";
import { requestIdOf } from "../../src/log";
import { defineTool, type ToolDefinition } from "../../src/mcp/registry";
import { createMcpServer } from "../../src/mcp/server";
import { firstText, rpcTo } from "../helpers/mcp";

afterEach(() => vi.restoreAllMocks());

const cacheClear = defineTool({
  name: "cache_clear",
  title: "消去",
  description: "d",
  category: "Destructive",
  inputSchema: z.object({}),
  handler: async () => ({ cleared: true }),
});
const listThings = defineTool({
  name: "list_things",
  title: "一覧",
  description: "d",
  category: "ReadOnly",
  inputSchema: z.object({ refresh: z.enum(["none", "delta", "full"]).optional() }),
  handler: async () => ({ items: [] }),
});
const boom = defineTool({
  name: "boom_tool",
  title: "失敗",
  description: "d",
  category: "ReadOnly",
  inputSchema: z.object({}),
  handler: async () => {
    throw new Error("boom Bearer SECRETTOKEN");
  },
});

function spyAll() {
  return {
    info: vi.spyOn(console, "info").mockImplementation(() => {}),
    warn: vi.spyOn(console, "warn").mockImplementation(() => {}),
    error: vi.spyOn(console, "error").mockImplementation(() => {}),
    log: vi.spyOn(console, "log").mockImplementation(() => {}),
  };
}

/** console.* に渡された全ての行を JSON として読む(1 行 = 1 つの JSON オブジェクト)。 */
function lines(spies: ReturnType<typeof spyAll>): Record<string, unknown>[] {
  return Object.values(spies).flatMap((s) =>
    s.mock.calls.map((c) => {
      expect(c).toHaveLength(1);
      return JSON.parse(String(c[0])) as Record<string, unknown>;
    }),
  );
}

async function call(tool: ToolDefinition, args: Record<string, unknown>, actor?: string) {
  return rpcTo(
    () => createMcpServer({ tools: [tool], requestId: "ray-123", ...(actor ? { actor } : {}) }),
    "tools/call",
    { name: tool.name, arguments: args },
  );
}

describe("requestIdOf", () => {
  it("cf-ray があればそれを使う", () => {
    const req = new Request("https://x.test/", { headers: { "cf-ray": "abc123-NRT" } });
    expect(requestIdOf(req)).toBe("abc123-NRT");
  });
  it("なければ UUID を作る", () => {
    expect(requestIdOf(new Request("https://x.test/"))).toMatch(/^[0-9a-f-]{36}$/);
  });
});

describe("書き込み系の操作のログ", () => {
  it("cache_clear は、ツール名・主体・request_id を JSON 1 行で残す", async () => {
    const spies = spyAll();
    await call(cacheClear, {}, "alice@example.com");
    const entries = lines(spies).filter((l) => l.event === "write_operation");
    expect(entries).toHaveLength(1);
    expect(entries[0]).toMatchObject({
      tool: "cache_clear",
      actor: "alice@example.com",
      request_id: "ray-123",
    });
  });

  it.each(["delta", "full"])("refresh=%s の読み取りツールも残す", async (refresh) => {
    const spies = spyAll();
    await call(listThings, { refresh }, "alice@example.com");
    const entries = lines(spies).filter((l) => l.event === "write_operation");
    expect(entries).toHaveLength(1);
    expect(entries[0]).toMatchObject({
      tool: "list_things",
      refresh,
      actor: "alice@example.com",
      request_id: "ray-123",
    });
  });

  it.each([{}, { refresh: "none" }])("読み取りだけ(%j)では主体を出さない", async (args) => {
    const spies = spyAll();
    await call(listThings, args, "alice@example.com");
    const out = JSON.stringify(lines(spies));
    expect(out).not.toContain("alice@example.com");
    expect(lines(spies).filter((l) => l.event === "write_operation")).toHaveLength(0);
  });
});

describe("想定外の例外のログ", () => {
  it("request_id・ツール名が入り、秘密は伏せる。主体は読み取りなので出さない", async () => {
    const spies = spyAll();
    const res = await call(boom, {}, "alice@example.com");
    expect(firstText(res)).not.toContain("SECRET");
    const entries = lines(spies).filter((l) => l.event === "tool_failed_unexpectedly");
    expect(entries).toHaveLength(1);
    expect(entries[0]).toMatchObject({ level: "error", tool: "boom_tool", request_id: "ray-123" });
    const out = JSON.stringify(lines(spies));
    expect(out).not.toContain("SECRETTOKEN");
    expect(out).not.toContain("alice@example.com");
  });
});

describe("主体の決め方(actorOf)", () => {
  it("人は email", () => {
    expect(actorOf({ kind: "user", email: "a@example.com", subject: "sub-1" })).toBe(
      "a@example.com",
    );
  });
  it("email が空で sub だけなら sub", () => {
    expect(actorOf({ kind: "user", email: "", subject: "sub-1" })).toBe("sub-1");
  });
  it("service token は clientId", () => {
    expect(actorOf({ kind: "service", clientId: "abc123.access" })).toBe("abc123.access");
  });
});

describe("想定外の例外のログの伏せ字(実値)", () => {
  const KEY = "  k/e y+&=?#%  ";
  const TOKEN = "tok:en@;,!*'()";
  const env = {
    BOARD_BASE_URL: "https://api.board.example",
    BOARD_API_KEY: KEY,
    BOARD_API_TOKEN: TOKEN,
  } as never;
  const leaky = defineTool({
    name: "leaky",
    title: "漏れ",
    description: "d",
    category: "ReadOnly",
    inputSchema: z.object({}),
    handler: async () => {
      const k = KEY.trim();
      const t = TOKEN.trim();
      throw new Error(`raw ${k} ${t} enc ${encodeURIComponent(k)} ${encodeURIComponent(t)} end`);
    },
  });

  it("実値と encodeURIComponent した形を伏せる", async () => {
    const spies = spyAll();
    await rpcTo(() => createMcpServer({ tools: [leaky], requestId: "r", env }), "tools/call", {
      name: "leaky",
      arguments: {},
    });
    const entry = lines(spies).find((l) => l.event === "tool_failed_unexpectedly");
    const msg = String(entry?.error_message);
    const k = KEY.trim();
    const t = TOKEN.trim();
    for (const v of [k, t, encodeURIComponent(k), encodeURIComponent(t)]) {
      expect(msg).not.toContain(v);
    }
    expect(msg).toContain("[REDACTED]");
    expect(msg).toContain("raw");
    expect(msg).toContain("end");
  });

  it("error_message は 500 文字で切る", async () => {
    const long = defineTool({
      name: "long_err",
      title: "長い",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async () => {
        throw new Error("x".repeat(2000));
      },
    });
    const spies = spyAll();
    await call(long, {});
    const entry = lines(spies).find((l) => l.event === "tool_failed_unexpectedly");
    expect(String(entry?.error_message).length).toBeLessThanOrEqual(500);
  });
});
