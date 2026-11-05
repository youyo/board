import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { ArgumentError, BoardError } from "../../src/errors";
import { annotationsFor, defineTool, type ToolDefinition } from "../../src/mcp/registry";
import { createMcpServer } from "../../src/mcp/server";
import { firstText, rpcTo } from "../helpers/mcp";

afterEach(() => vi.restoreAllMocks());

async function call(tool: ToolDefinition, args: unknown) {
  const res = await rpcTo(() => createMcpServer({ tools: [tool] }), "tools/call", {
    name: tool.name,
    arguments: args as Record<string, unknown>,
  });
  const text = firstText(res);
  return { text, isError: res.result?.isError === true, json: JSON.parse(text) };
}

const echo = defineTool({
  name: "echo_text",
  title: "エコー",
  description: "Echo",
  category: "ReadOnly",
  inputSchema: z.object({ text: z.string() }),
  handler: async ({ text }) => ({ echoed: text }),
});

describe("annotationsFor", () => {
  it("ReadOnly は readOnly / idempotent / openWorld", () => {
    expect(annotationsFor("ReadOnly", "t")).toEqual({
      title: "t",
      readOnlyHint: true,
      idempotentHint: true,
      openWorldHint: true,
    });
  });
  it("Destructive は destructiveHint=true", () => {
    expect(annotationsFor("Destructive", "t").destructiveHint).toBe(true);
  });
});

describe("defineTool", () => {
  it("名前は snake_case(接頭辞なし)", () => {
    const base = {
      title: "t",
      description: "d",
      category: "ReadOnly" as const,
      inputSchema: z.object({}),
      handler: async () => ({}),
    };
    expect(() => defineTool({ ...base, name: "Server-Info" })).toThrow();
    expect(() => defineTool({ ...base, name: "server info" })).toThrow();
    expect(() => defineTool({ ...base, name: "" })).toThrow();
    expect(defineTool({ ...base, name: "find_projects" }).name).toBe("find_projects");
  });
});

describe("ツール呼び出し", () => {
  it("成功すると JSON を text content で返す", async () => {
    const r = await call(echo, { text: "hi" });
    expect(r.isError).toBe(false);
    expect(r.json).toEqual({ echoed: "hi" });
  });

  it("入力が不正なら argument_error のエンベロープ(isError)", async () => {
    const r = await call(echo, { text: 1 });
    expect(r.isError).toBe(true);
    expect(r.json.schema_version).toBe("1");
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.retryable).toBe(false);
    expect(r.json.error.message).toContain("text");
  });

  it("ハンドラが BoardError を投げるとそのコードのエンベロープになる", async () => {
    const t = defineTool({
      name: "fail_board",
      title: "失敗",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async () => {
        throw new BoardError("rate_limited", "slow down", true);
      },
    });
    const r = await call(t, {});
    expect(r.isError).toBe(true);
    expect(r.json.error).toEqual({ code: "rate_limited", message: "slow down", retryable: true });
  });

  it("ハンドラが ArgumentError を投げると argument_error", async () => {
    const t = defineTool({
      name: "fail_arg",
      title: "失敗",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async () => {
        throw new ArgumentError("nope");
      },
    });
    expect((await call(t, {})).json.error.code).toBe("argument_error");
  });

  it("想定外の例外は generic_error で、message は応答に出さず、ログにも秘密を出さない", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const t = defineTool({
      name: "fail_unexpected",
      title: "失敗",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async () => {
        throw new Error("boom x-api-key: SECRETKEY Bearer SECRETTOKEN");
      },
    });
    const r = await call(t, {});
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("generic_error");
    expect(r.text).not.toContain("SECRET");
    const logged = spy.mock.calls.map((c) => c.join(" ")).join("\n");
    expect(logged).toContain("fail_unexpected");
    expect(logged).not.toContain("SECRETTOKEN");
  });

  it("ハンドラの ctx に MCP のリクエストの abort signal が渡る", async () => {
    let seen: unknown;
    const t = defineTool({
      name: "probe_signal",
      title: "確認",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async (_args, ctx) => {
        seen = ctx.signal;
        return {};
      },
    });
    await call(t, {});
    expect(seen).toBeInstanceOf(AbortSignal);
  });

  it("repositories が未設定のサーバーでは、ctx.repositories() は例外(generic_error。秘密は出さない)", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const t = defineTool({
      name: "probe_repos",
      title: "確認",
      description: "d",
      category: "ReadOnly",
      inputSchema: z.object({}),
      handler: async (_args, ctx) => ctx.repositories(),
    });
    const r = await call(t, {});
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("generic_error");
  });

  it("ツール名が重複していると createMcpServer が例外", () => {
    expect(() => createMcpServer({ tools: [echo, echo] })).toThrow(/echo_text/);
  });
});
