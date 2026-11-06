import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { requestIdOf } from "../../src/log";
import { defineTool, type ToolDefinition } from "../../src/mcp/registry";
import { createMcpServer } from "../../src/mcp/server";
import { firstText, rpcTo } from "../helpers/mcp";

afterEach(() => vi.restoreAllMocks());

const JWT = ["fake-header", "fake-payload", "SIGNATURE_PART"].join(".");

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

describe("JWT をログに出さない", () => {
  it("書き込みのログにも JWT 文字列は含まれない", async () => {
    const spies = spyAll();
    await call(cacheClear, {}, "alice@example.com");
    expect(JSON.stringify(lines(spies))).not.toContain(JWT);
    expect(JSON.stringify(lines(spies))).not.toContain("SIGNATURE_PART");
  });
});
