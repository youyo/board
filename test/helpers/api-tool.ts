import { createMcpServer } from "../../src/mcp/server";
import { createRepositories } from "../../src/repository";
import { BASE, testClient } from "./board-list";
import { testDb } from "./d1";
import { firstText, rpcTo } from "./mcp";
import { mockOutboundFetch, type RecordedCall } from "./mock-fetch";

export { BASE };

export interface ApiToolRun {
  isError: boolean;
  text: string;
  // biome-ignore lint/suspicious/noExplicitAny: テストが応答の JSON を自由に辿るため
  json: any;
  calls: RecordedCall[];
}

/** 呼び出しの `GET /v1/contacts?a=1&b=2` 形式の表記(page / per_page は除き、キー順)。 */
export function describeApiCall(call: RecordedCall): string {
  const url = new URL(call.url);
  const pairs = [...url.searchParams.entries()]
    .filter(([k]) => k !== "page" && k !== "per_page")
    .map((entry, i) => ({ entry, i }))
    .sort((a, b) => (a.entry[0] < b.entry[0] ? -1 : a.entry[0] > b.entry[0] ? 1 : a.i - b.i))
    .map(({ entry: [k, v] }) => `${k}=${v}`);
  return `${call.method} ${url.pathname}${pairs.length ? `?${pairs.join("&")}` : ""}`;
}

/**
 * 実物の repository(miniflare の D1 + 本物の BOARD クライアント。fetch はモック)を通してツールを呼ぶ。
 * handlers のキーは "GET https://api.board.test/v1/clients"(クエリなし)。未登録の宛先への通信は例外になる。
 */
export async function callApiTool(
  name: string,
  args: Record<string, unknown>,
  handlers: Parameters<typeof mockOutboundFetch>[0] = {},
): Promise<ApiToolRun> {
  const mocks = mockOutboundFetch(handlers);
  const res = await rpcTo(
    () =>
      createMcpServer({
        repositories: () => createRepositories({ db: testDb(), client: testClient() }),
      }),
    "tools/call",
    { name, arguments: args },
  );
  const text = firstText(res);
  let json: unknown;
  try {
    json = JSON.parse(text);
  } catch {
    json = undefined;
  }
  return { isError: res.result?.isError === true, text, json, calls: mocks.calls };
}
