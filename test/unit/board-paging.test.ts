import { describe, expect, it } from "vitest";
import { DEFAULT_MAX_PAGES, DEFAULT_PER_PAGE, fetchAllPages } from "../../src/board/paging";
import { ArgumentError, BoardError } from "../../src/errors";

const h = (n: number) => new Headers({ "X-Page": String(n) });

/** perPage 件ずつ、全部で total 件を返すページ取得関数。 */
function source(total: number) {
  const calls: Array<{ page: number; perPage: number }> = [];
  const fetchPage = async (page: number, perPage: number) => {
    calls.push({ page, perPage });
    const start = (page - 1) * perPage;
    const items = Array.from(
      { length: Math.max(0, Math.min(perPage, total - start)) },
      (_, i) => start + i + 1,
    );
    return { items, headers: h(page) };
  };
  return { calls, fetchPage };
}

// Go の internal/boardapi/pagination_test.go の転記 + maxPages。
describe("fetchAllPages (Go の ListAllWithResult)", () => {
  it("既定値", () => {
    expect(DEFAULT_PER_PAGE).toBe(100);
    expect(DEFAULT_MAX_PAGES).toBe(10);
  });

  it("per_page 未満のページで終わる。ヘッダーは最終ページのもの", async () => {
    const s = source(250);
    const r = await fetchAllPages(s.fetchPage);
    expect(r.items).toHaveLength(250);
    expect(r.items[0]).toBe(1);
    expect(r.items[249]).toBe(250);
    expect(r.truncated).toBe(false);
    expect(r.pages).toBe(3);
    expect(r.headers?.get("x-page")).toBe("3");
    expect(s.calls).toEqual([
      { page: 1, perPage: 100 },
      { page: 2, perPage: 100 },
      { page: 3, perPage: 100 },
    ]);
  });

  it("ちょうど割り切れるときは次の(空の)ページまで取りに行く", async () => {
    const s = source(200);
    const r = await fetchAllPages(s.fetchPage);
    expect(r.items).toHaveLength(200);
    expect(r.pages).toBe(3);
    expect(r.truncated).toBe(false);
    expect(s.calls.map((c) => c.page)).toEqual([1, 2, 3]);
  });

  it("空の応答は 1 ページで終わる", async () => {
    const s = source(0);
    const r = await fetchAllPages(s.fetchPage);
    expect(r.items).toEqual([]);
    expect(r.pages).toBe(1);
    expect(r.truncated).toBe(false);
  });

  it("perPage を指定できる(page と per_page を渡す)", async () => {
    const s = source(5);
    const r = await fetchAllPages(s.fetchPage, { perPage: 2 });
    expect(r.items).toEqual([1, 2, 3, 4, 5]);
    expect(s.calls).toEqual([
      { page: 1, perPage: 2 },
      { page: 2, perPage: 2 },
      { page: 3, perPage: 2 },
    ]);
  });

  it("maxPages で止め、続きがありそうなら truncated=true", async () => {
    const s = source(10_000);
    const r = await fetchAllPages(s.fetchPage, { perPage: 100, maxPages: 3 });
    expect(r.items).toHaveLength(300);
    expect(r.pages).toBe(3);
    expect(r.truncated).toBe(true);
    expect(s.calls).toHaveLength(3);
  });

  it("既定の maxPages は 10 ページ(1,000 件)", async () => {
    const s = source(10_000);
    const r = await fetchAllPages(s.fetchPage);
    expect(r.items).toHaveLength(1000);
    expect(r.truncated).toBe(true);
  });

  it("maxPages ちょうどで最後のページが短ければ truncated=false", async () => {
    const s = source(250);
    const r = await fetchAllPages(s.fetchPage, { maxPages: 3 });
    expect(r.items).toHaveLength(250);
    expect(r.truncated).toBe(false);
  });

  it("ページは順番に取る(並列にしない)", async () => {
    let inFlight = 0;
    let max = 0;
    const r = await fetchAllPages(
      async (page, perPage) => {
        inFlight++;
        max = Math.max(max, inFlight);
        await new Promise((res) => setTimeout(res, 5));
        inFlight--;
        return { items: page < 3 ? new Array(perPage).fill(page) : [], headers: h(page) };
      },
      { perPage: 3 },
    );
    expect(max).toBe(1);
    expect(r.items).toHaveLength(6);
  });

  it("途中のエラーはそのまま伝わる", async () => {
    const err = new BoardError("api_error", "boom", true);
    const e = await fetchAllPages(async (page) => {
      if (page === 2) throw err;
      return { items: new Array(100).fill(1), headers: h(page) };
    }).catch((x) => x);
    expect(e).toBe(err);
  });

  it("perPage・maxPages は正の整数だけ(それ以外は ArgumentError)。perPage<=0 は未指定扱い(Go と同じ)", async () => {
    const s = source(3);
    for (const opts of [{ perPage: 1.5 }, { maxPages: 0 }, { maxPages: -1 }, { maxPages: 2.5 }]) {
      await expect(fetchAllPages(s.fetchPage, opts)).rejects.toBeInstanceOf(ArgumentError);
    }
    const r = await fetchAllPages(s.fetchPage, { perPage: 0 });
    expect(r.items).toHaveLength(3);
    expect(s.calls.at(-1)?.perPage).toBe(100);
  });
});
