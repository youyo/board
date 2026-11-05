import { createBoardClient } from "../../src/board/client";
import { jsonResponse, type RecordedCall } from "./mock-fetch";

export const BASE = "https://api.board.test";

/** page / per_page で配列を切って返す BOARD の一覧のモック。`onPage` で応答の差し替え・遅延・失敗を仕込める。 */
export function pagedHandler(
  rows: () => unknown[],
  onPage?: (
    page: number,
    q: URLSearchParams,
    call: RecordedCall,
  ) => Response | Promise<Response | undefined> | undefined,
) {
  return async (call: RecordedCall): Promise<Response> => {
    const q = new URL(call.url).searchParams;
    const page = Number(q.get("page") ?? "1");
    const perPage = Number(q.get("per_page") ?? "100");
    const over = onPage ? await onPage(page, q, call) : undefined;
    if (over) return over;
    const all = rows();
    return jsonResponse(all.slice((page - 1) * perPage, page * perPage));
  };
}

export function makeEntities(
  n: number,
  extra: (i: number) => Record<string, unknown> = () => ({}),
) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    name: `顧客${i + 1}`,
    updated_at: `2026-01-01T00:00:${String(i % 60).padStart(2, "0")}.000+09:00`,
    ...extra(i),
  }));
}

export function testClient(now?: () => number) {
  return createBoardClient({
    baseUrl: BASE,
    apiKey: "k",
    apiToken: "t",
    maxRetries: 0,
    sleep: async () => {},
    ...(now ? { now } : {}),
  });
}
