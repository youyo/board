import type { McpServer } from "@modelcontextprotocol/server";
import { createMcpHandler } from "agents/mcp/server";
import { expect } from "vitest";

export interface RpcResponse {
  result?: {
    tools?: {
      name: string;
      description?: string;
      inputSchema?: { properties?: Record<string, unknown> };
      annotations?: { readOnlyHint?: boolean };
    }[];
    content?: { type: string; text: string }[];
    isError?: boolean;
    serverInfo?: { name: string; version: string };
  };
  error?: { code: number; message: string };
}

/** 応答は JSON または SSE(`data: {...}` 行)のどちらでも来る。 */
export function parseRpcResponse(text: string): RpcResponse {
  const data = text.startsWith("{")
    ? text
    : text
        .split("\n")
        .find((l) => l.startsWith("data:"))
        ?.slice(5)
        .trim();
  return JSON.parse(data ?? "null");
}

export async function readRpc(res: Response): Promise<RpcResponse> {
  return parseRpcResponse(await res.text());
}

/** tools/call の最初の text content を取り出す。 */
export function firstText(rpc: RpcResponse): string {
  return rpc.result?.content?.[0]?.text ?? "";
}

export const MCP_HEADERS = {
  "Content-Type": "application/json",
  Accept: "application/json, text/event-stream",
};

/** McpServer のファクトリに JSON-RPC を 1 件送り、応答を返す(HTTP 層は createMcpHandler を通る)。 */
export async function rpcTo(
  factory: () => McpServer,
  method: string,
  params: Record<string, unknown> = {},
): Promise<RpcResponse> {
  const handler = createMcpHandler(factory, { route: "/mcp" });
  const res = await handler.fetch(
    new Request("https://board.test/mcp", {
      method: "POST",
      headers: MCP_HEADERS,
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
    }),
  );
  const text = await res.text();
  expect(res.status, text.slice(0, 300)).toBe(200);
  return parseRpcResponse(text);
}
