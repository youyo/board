import { afterEach, describe, expect, it, vi } from "vitest";
import { createBoardClient } from "../../src/board/client";
import { listClients } from "../../src/board/resources";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => {
  vi.restoreAllMocks();
});

// Worker 全体を通すツールはまだない(M04)ので、モック fetch に対して
// 「list → ページング → 全件」を通しで確認する。
describe("list → ページング → 全件(モック fetch)", () => {
  const TOTAL = 230;
  const all = Array.from({ length: TOTAL }, (_, i) => ({
    id: i + 1,
    name: `顧客${i + 1}`,
    future_field: { n: i },
  }));

  function setup(opts: { failFirstPage2?: boolean } = {}) {
    let page2Failed = false;
    return mockOutboundFetch({
      "GET https://api.board.test/v1/clients": (call) => {
        const q = new URL(call.url).searchParams;
        const page = Number(q.get("page"));
        const perPage = Number(q.get("per_page"));
        if (opts.failFirstPage2 && page === 2 && !page2Failed) {
          page2Failed = true;
          return new Response(JSON.stringify({ message: "rate limited" }), {
            status: 429,
            headers: { "Retry-After": "1" },
          });
        }
        return new Response(JSON.stringify(all.slice((page - 1) * perPage, page * perPage)), {
          headers: {
            "Content-Type": "application/json",
            "X-Total-Count": String(TOTAL),
            "X-Page": String(page),
            "X-Per-Page": String(perPage),
            "X-Ratelimit-Remaining": String(3000 - page),
            "X-Ratelimit-Limit": "3000",
          },
        });
      },
    });
  }

  const newClient = (waits: number[] = []) =>
    createBoardClient({
      baseUrl: "https://api.board.test",
      apiKey: "key-1",
      apiToken: "token-1",
      sleep: async (ms) => {
        waits.push(ms);
      },
    });

  it("3 ページを順に取り、全件と最終ページの _meta を返す", async () => {
    const m = setup();
    const r = await listClients(newClient(), { nameCont: "顧客", includeArchiveFlg: false });
    expect(r.items).toHaveLength(TOTAL);
    expect(r.items[0]).toEqual({ id: 1, name: "顧客1", future_field: { n: 0 } });
    expect(r.items[TOTAL - 1]).toEqual({
      id: TOTAL,
      name: `顧客${TOTAL}`,
      future_field: { n: TOTAL - 1 },
    });
    expect(r.truncated).toBe(false);
    expect(r._meta).toEqual({
      total_count: 230,
      page: 3,
      per_page: 100,
    });
    expect(m.calls.map((c) => c.url)).toEqual(
      [1, 2, 3].map(
        (p) =>
          `https://api.board.test/v1/clients?include_archive_flg=0&name_cont=%E9%A1%A7%E5%AE%A2&page=${p}&per_page=100`,
      ),
    );
    for (const c of m.calls) {
      expect(c.headers.get("x-api-key")).toBe("key-1");
      expect(c.headers.get("authorization")).toBe("Bearer token-1");
    }
  });

  it("途中で 429 が返っても Retry-After を待って続きから取り、全件そろう", async () => {
    const m = setup({ failFirstPage2: true });
    const waits: number[] = [];
    const r = await listClients(newClient(waits), {});
    expect(r.items).toHaveLength(TOTAL);
    expect(waits).toEqual([1000]);
    expect(m.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual([
      "1",
      "2",
      "2",
      "3",
    ]);
  });

  it("maxPages で打ち切ると truncated=true と、取れた分だけを返す", async () => {
    setup();
    const r = await listClients(newClient(), {}, { maxPages: 2 });
    expect(r.items).toHaveLength(200);
    expect(r.truncated).toBe(true);
    expect(r._meta.page).toBe(2);
  });

  it("途中のページが 500 を返し続けるとエラー(エンベロープ用の code と retryable)", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": (call) =>
        new URL(call.url).searchParams.get("page") === "1"
          ? jsonResponse(all.slice(0, 100))
          : jsonResponse({ message: "boom" }, 500),
    });
    const e = await listClients(
      createBoardClient({
        baseUrl: "https://api.board.test",
        apiKey: "k",
        apiToken: "t",
        maxRetries: 1,
        sleep: async () => {},
      }),
      {},
    ).catch((x) => x);
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(true);
  });
});
