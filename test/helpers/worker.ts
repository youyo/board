import { env } from "cloudflare:workers";
import type { Env } from "../../src/env";
import worker from "../../src/index";
import { MCP_HEADERS } from "./mcp";

export const ORIGIN = "https://board.test";
export const MCP_URL = `${ORIGIN}/mcp`;

const ctx = { waitUntil() {}, passThroughOnException() {} } as unknown as ExecutionContext;

/**
 * Worker 全体(src/index.ts の default export)を呼ぶ。`overrides` で env を差し替える
 * (undefined を渡すとその項目を欠かす)。`jwt` があれば Cf-Access-Jwt-Assertion に付ける。
 */
export function callWorker(
  path: string,
  opts: { overrides?: Partial<Record<keyof Env, unknown>>; jwt?: string; init?: RequestInit } = {},
): Promise<Response> {
  const testEnv = { ...env, ...opts.overrides } as unknown as Env;
  const headers = new Headers(opts.init?.headers);
  if (opts.jwt) headers.set("Cf-Access-Jwt-Assertion", opts.jwt);
  return worker.fetch(
    new Request(`${ORIGIN}${path}`, { redirect: "manual", ...opts.init, headers }),
    testEnv,
    ctx,
  );
}

/** /mcp への JSON-RPC 1 件。 */
export function mcpCall(
  jwt: string | undefined,
  method: string,
  params: Record<string, unknown> = {},
  id = 1,
): Promise<Response> {
  return callWorker("/mcp", {
    ...(jwt ? { jwt } : {}),
    init: {
      method: "POST",
      headers: MCP_HEADERS,
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
    },
  });
}
