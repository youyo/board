/**
 * 応答サイズの上限(安全網。logbutler の ADR 0007 と同じ)。ツール引数 max_response_tokens(既定 50000、0 で無効)。
 * 超えたら切り詰めず response_too_large(retryable: false)のエンベロープを返す。対象は読み取りツール。
 */
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { defineTool, type ToolDefinition } from "../../src/mcp/registry";
import { DEFAULT_MAX_RESPONSE_TOKENS, estimateTokens } from "../../src/mcp/response-limit";
import { createMcpServer } from "../../src/mcp/server";
import { firstText, rpcTo } from "../helpers/mcp";

const reader = defineTool({
  name: "big_read",
  title: "大きな応答",
  description: "Returns a big payload",
  category: "ReadOnly",
  inputSchema: z.object({ chars: z.number() }),
  handler: async ({ chars }) => ({ text: "a".repeat(chars) }),
});

const noLimitReader = defineTool({
  name: "big_read_nolimit",
  title: "上限なし",
  description: "Returns a big payload without the limit",
  category: "ReadOnly",
  responseLimit: false,
  inputSchema: z.object({ chars: z.number() }),
  handler: async ({ chars }) => ({ text: "a".repeat(chars) }),
});

const writer = defineTool({
  name: "big_write",
  title: "大きな応答の書き込み",
  description: "Writes and returns a big payload",
  category: "WriteIdempotent",
  inputSchema: z.object({ chars: z.number() }),
  handler: async ({ chars }) => ({ text: "a".repeat(chars) }),
});

async function call(tool: ToolDefinition, args: Record<string, unknown>) {
  const res = await rpcTo(() => createMcpServer({ tools: [tool] }), "tools/call", {
    name: tool.name,
    arguments: args,
  });
  const text = firstText(res);
  return { text, isError: res.result?.isError === true, json: JSON.parse(text) };
}

const payload = (chars: number) => JSON.stringify({ text: "a".repeat(chars) });

describe("estimateTokens(近似: ASCII は 4 文字 1 トークン、ASCII 以外は 1 文字 1 トークン)", () => {
  it("ASCII は 4 文字ごとに 1(切り上げ)", () => {
    expect(estimateTokens("")).toBe(0);
    expect(estimateTokens("abcd")).toBe(1);
    expect(estimateTokens("abcde")).toBe(2);
  });

  it("日本語など ASCII 以外は 1 文字 1 トークンで数える", () => {
    expect(estimateTokens("あいうえお")).toBe(5);
    expect(estimateTokens("abcdあい")).toBe(3);
  });
});

describe("response_too_large", () => {
  it("既定値は 50000", () => {
    expect(DEFAULT_MAX_RESPONSE_TOKENS).toBe(50000);
  });

  it("読み取りツールの入力スキーマに max_response_tokens が付く", () => {
    expect(Object.keys(reader.inputSchema.shape)).toContain("max_response_tokens");
    expect(Object.keys(noLimitReader.inputSchema.shape)).not.toContain("max_response_tokens");
    expect(Object.keys(writer.inputSchema.shape)).not.toContain("max_response_tokens");
  });

  it("上限ちょうどは通り、1 超えるとエラー(切り詰めない)", async () => {
    const chars = 400;
    const size = estimateTokens(payload(chars));
    const ok = await call(reader, { chars, max_response_tokens: size });
    expect(ok.isError).toBe(false);
    expect(ok.json.text).toBe("a".repeat(chars));

    const over = await call(reader, { chars, max_response_tokens: size - 1 });
    expect(over.isError).toBe(true);
    expect(over.json.schema_version).toBe("1");
    expect(over.json.error.code).toBe("response_too_large");
    expect(over.json.error.retryable).toBe(false);
    expect(over.text).not.toContain("aaaa");
  });

  it("message に推定サイズ・上限・max_response_tokens の手がかりを含む", async () => {
    const size = estimateTokens(payload(400));
    const over = await call(reader, { chars: 400, max_response_tokens: 10 });
    const msg: string = over.json.error.message;
    expect(msg).toContain(String(size));
    expect(msg).toContain("10");
    expect(msg.toLowerCase()).toContain("estimated");
    expect(msg).toContain("max_response_tokens");
  });

  it("引数がなければ既定(50000)が効く", async () => {
    // JSON {"text":"..."} の枠 11 文字 + 本文。ちょうど 50000 トークン(= 200000 文字)になる本文長。
    const fit = 200000 - 11;
    expect(estimateTokens(payload(fit))).toBe(50000);
    expect((await call(reader, { chars: fit })).isError).toBe(false);
    const over = await call(reader, { chars: fit + 4 });
    expect(over.isError).toBe(true);
    expect(over.json.error.code).toBe("response_too_large");
  });

  it("0 で無効: 既定を超える応答も通る", async () => {
    const res = await call(reader, { chars: 250000, max_response_tokens: 0 });
    expect(res.isError).toBe(false);
  });

  it("負の値・小数は argument_error", async () => {
    const neg = await call(reader, { chars: 1, max_response_tokens: -1 });
    expect(neg.isError).toBe(true);
    expect(neg.json.error.code).toBe("argument_error");
    const frac = await call(reader, { chars: 1, max_response_tokens: 1.5 });
    expect(frac.isError).toBe(true);
    expect(frac.json.error.code).toBe("argument_error");
  });

  it("responseLimit: false のツールと、書き込みツールは上限の対象外", async () => {
    expect((await call(noLimitReader, { chars: 250000 })).isError).toBe(false);
    expect((await call(writer, { chars: 250000 })).isError).toBe(false);
  });
});
