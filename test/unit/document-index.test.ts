/**
 * 書類 ID → 案件 ID の逆引き(M05)。Go 版 `reverse_map_test.go` のケースを、Workers の方式(D1 の表 + 再開できる走査)に読み替えたもの。
 * Go は projects を全件走査して 10 秒を超えたら ProjectID=0 に縮退していた。ここでは走査の進捗を D1 に残し、
 * 締め切りの手前で止めて『未解決』を返す(次の呼び出しが続きから進む)。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createBoardClient } from "../../src/board/client";
import { CACHE_QUERY_VERSION } from "../../src/cache/sync-state";
import {
  COMPLETE_TTL_MS,
  createDocumentIndex,
  MAX_SCAN_PAGES_PER_CALL,
  SCAN_LOCK_STALE_MS,
  SCAN_RESERVE_MS,
} from "../../src/repository/document-index";
import { DOCUMENT_KIND_INFO } from "../../src/repository/document-kinds";
import { BASE, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const projectsKey = `GET ${BASE}/v1/projects`;
const PER_PAGE = 2;

/** 案件 n 件。案件 i の見積 ID = 1000 + i、納品 ID = [2000 + i, 3000 + i]。 */
const projects = (n: number) =>
  Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    name: `案件${i + 1}`,
    estimate: { id: 1000 + i + 1 },
    deliveries: [{ id: 2000 + i + 1 }, { id: 3000 + i + 1 }],
  }));

function setup(opts: { perPage?: number } = {}) {
  const clock = fakeClock();
  const index = createDocumentIndex({
    db: testDb(),
    client: testClient(clock.now),
    now: clock.now,
    perPage: opts.perPage ?? PER_PAGE,
  });
  return { clock, index, db: testDb() };
}
const pagesOf = (calls: { url: string }[]) =>
  calls.map((c) => Number(new URL(c.url).searchParams.get("page")));
const count = async (sql: string) => (await testDb().prepare(sql).first<{ c: number }>())?.c ?? 0;

describe("record / lookup の基本", () => {
  it("記録済みの書類は通信せずに案件 ID を返す", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({});
    await index.record("estimate", [{ id: 7, estimate: { id: 77 } }] as never);
    expect(await index.lookup("estimate", 77)).toEqual({ status: "hit", projectId: 7 });
    expect(api.calls).toHaveLength(0);
  });

  it("record は種別ごとに該当する書類 ID だけを覚える(納品は配列の全要素)", async () => {
    const { index } = setup();
    mockOutboundFetch({});
    await index.record("delivery", [
      { id: 5, estimate: { id: 50 }, deliveries: [{ id: 51 }, { id: 52 }] },
    ] as never);
    expect(await index.lookup("delivery", 51)).toEqual({ status: "hit", projectId: 5 });
    expect(await index.lookup("delivery", 52)).toEqual({ status: "hit", projectId: 5 });
    // 見積 ID は delivery としては覚えていない(走査に進むが、通信のモックがないので解決できない)。
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(2);
  });
});

describe("走査(表にない書類)", () => {
  it("response_group=<種別> で先頭ページから走査し、見つけた時点で止める。進捗(次のページ)を残す", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    // 見積 1005 は案件 5 = 3 ページ目。
    expect(await index.lookup("estimate", 1005)).toEqual({ status: "hit", projectId: 5 });
    expect(pagesOf(api.calls)).toEqual([1, 2, 3]);
    for (const c of api.calls) {
      expect(new URL(c.url).searchParams.get("response_group")).toBe("estimate");
    }
    // 走査したページの案件はすべて表に入っている(案件 1〜6 の見積)。
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(6);
    const st = await testDb()
      .prepare("SELECT * FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ next_page: number; completed_at: string | null; lock_owner: string | null }>();
    expect(st?.next_page).toBe(4);
    expect(st?.completed_at).toBeNull();
    expect(st?.lock_owner).toBeNull();
  });

  it("次の呼び出しは保存した次のページから続ける(先頭からやり直さない)。表にあれば通信しない", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    await index.lookup("estimate", 1005);
    api.calls.length = 0;
    expect(await index.lookup("estimate", 1003)).toEqual({ status: "hit", projectId: 3 });
    expect(api.calls).toHaveLength(0);
    expect(await index.lookup("estimate", 1009)).toEqual({ status: "hit", projectId: 9 });
    expect(pagesOf(api.calls)).toEqual([4, 5]);
  });

  it("最後のページまで見て見つからなければ not_found。TTL の間は再走査しない", async () => {
    const { index, clock } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(5)) });
    expect(await index.lookup("estimate", 99999)).toEqual({ status: "miss", reason: "not_found" });
    expect(pagesOf(api.calls)).toEqual([1, 2, 3]);
    api.calls.length = 0;
    clock.advance(COMPLETE_TTL_MS - 1000);
    expect(await index.lookup("estimate", 99999)).toEqual({ status: "miss", reason: "not_found" });
    expect(api.calls).toHaveLength(0);
  });

  it("TTL を過ぎた miss は先頭から走査し直す(走査のあとに作られた書類を拾う)", async () => {
    const { index, clock } = setup();
    let rows = projects(3);
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => rows) });
    await index.lookup("estimate", 5555);
    clock.advance(COMPLETE_TTL_MS + 1000);
    rows = [...rows, { id: 4, name: "新", estimate: { id: 5555 }, deliveries: [] }];
    api.calls.length = 0;
    expect(await index.lookup("estimate", 5555)).toEqual({ status: "hit", projectId: 4 });
    expect(pagesOf(api.calls)).toEqual([1, 2]);
  });

  it("納品は配列の全要素を覚える(同じ案件の複数の納品書)", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(4)) });
    expect(await index.lookup("delivery", 3002)).toEqual({ status: "hit", projectId: 2 });
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("response_group")).toBe("delivery");
    expect(await index.lookup("delivery", 2001)).toEqual({ status: "hit", projectId: 1 });
    expect(api.calls).toHaveLength(1);
  });

  it("種別ごとに走査の進捗は独立している", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    await index.lookup("estimate", 1005);
    api.calls.length = 0;
    await index.lookup("order", 4242);
    expect(pagesOf(api.calls)[0]).toBe(1);
  });
});

describe("締め切り(縮退)", () => {
  it("走査に使える時間(残り - 予約)が最小に満たなければ、通信せず unresolved(deadline)", async () => {
    const { index, clock } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    const ctx = { deadline: clock.now() + SCAN_RESERVE_MS + 1000 };
    expect(await index.lookup("estimate", 1005, ctx)).toEqual({
      status: "unresolved",
      reason: "deadline",
    });
    expect(api.calls).toHaveLength(0);
  });

  it("走査の途中で時間が尽きたら止まり、取れたページの分は表に残る。次の呼び出しが続きから進む", async () => {
    const { index, clock } = setup();
    // 1 ページごとに 10 秒かかる。持ち時間 30 秒 - 予約 → 2 ページで尽きる。
    const api = mockOutboundFetch({
      [projectsKey]: pagedHandler(
        () => projects(20),
        () => {
          clock.advance(10_000);
          return undefined;
        },
      ),
    });
    const r = await index.lookup("estimate", 1019, { deadline: clock.now() + 30_000 });
    expect(r).toEqual({ status: "unresolved", reason: "deadline" });
    const done = pagesOf(api.calls);
    expect(done.length).toBeGreaterThanOrEqual(1);
    expect(done.length).toBeLessThanOrEqual(3);
    // 時間は予約を残して止まる(締め切りまで使い切らない)。
    expect(clock.now()).toBeLessThanOrEqual(
      Date.parse("2026-11-05T00:00:00Z") + 30_000 - SCAN_RESERVE_MS + 10_000,
    );
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(
      done.length * PER_PAGE,
    );

    api.calls.length = 0;
    const second = await index.lookup("estimate", 1019, { deadline: clock.now() + 200_000 });
    expect(second).toEqual({ status: "hit", projectId: 19 });
    expect(pagesOf(api.calls)[0]).toBe(done.length + 1);
  });

  it("ページ取得が締め切りに間に合わなかった(クライアントの締め切りエラー)も unresolved(deadline)", async () => {
    const clock = fakeClock();
    // 再試行できるクライアント: 応答のあとに締め切りを過ぎていれば、待たずに締め切りのエラーにする。
    const index = createDocumentIndex({
      db: testDb(),
      client: createBoardClient({
        baseUrl: BASE,
        apiKey: "k",
        apiToken: "t",
        maxRetries: 3,
        sleep: async () => {},
        now: clock.now,
      }),
      now: clock.now,
      perPage: PER_PAGE,
    });
    mockOutboundFetch({
      [projectsKey]: () => {
        clock.advance(60_000);
        return new Response(JSON.stringify({ message: "busy" }), {
          status: 503,
          headers: { "Retry-After": "1" },
        });
      },
    });
    const r = await index.lookup("estimate", 1, { deadline: clock.now() + 30_000 });
    expect(r).toEqual({ status: "unresolved", reason: "deadline" });
  });

  it("API のエラーは致命的にしない: unresolved(api_error)でエラーコードだけを返し、メッセージは載せない", async () => {
    const { index } = setup();
    mockOutboundFetch({
      [projectsKey]: () => jsonResponse({ message: "secret-ish text" }, 500),
    });
    const r = await index.lookup("estimate", 1);
    expect(r).toEqual({ status: "unresolved", reason: "api_error", errorCode: "api_error" });
    expect(JSON.stringify(r)).not.toContain("secret-ish");
  });
});

describe("ロック(同時の走査)", () => {
  it("別の呼び出しが走査中なら待たず unresolved(scan_in_progress)", async () => {
    const { index, db, clock } = setup();
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(3)) });
    await db
      .prepare(
        "INSERT INTO document_scan_state (document_kind, lock_started_at, lock_owner) VALUES ('estimate', ?, 'other')",
      )
      .bind(new Date(clock.now()).toISOString())
      .run();
    expect(await index.lookup("estimate", 1002)).toEqual({
      status: "unresolved",
      reason: "scan_in_progress",
    });
    expect(api.calls).toHaveLength(0);
  });

  it("stale なロックは奪って走査する。終わったら自分のロックを解放する", async () => {
    const { index, db, clock } = setup();
    mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(3)) });
    await db
      .prepare(
        "INSERT INTO document_scan_state (document_kind, lock_started_at, lock_owner) VALUES ('estimate', ?, 'other')",
      )
      .bind(new Date(clock.now() - SCAN_LOCK_STALE_MS - 1000).toISOString())
      .run();
    expect(await index.lookup("estimate", 1002)).toEqual({ status: "hit", projectId: 2 });
    const st = await db
      .prepare("SELECT lock_owner FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ lock_owner: string | null }>();
    expect(st?.lock_owner).toBeNull();
  });

  it("失敗しても(例外でも)ロックを解放する", async () => {
    const { index, db } = setup();
    mockOutboundFetch({ [projectsKey]: () => jsonResponse({}, 500) });
    await index.lookup("estimate", 1);
    const st = await db
      .prepare("SELECT lock_owner FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ lock_owner: string | null }>();
    expect(st?.lock_owner).toBeNull();
  });
});

/** prepare / bind / run に差し込める D1 のラッパー(実物の文を壊さず、プロパティだけを差し替える)。 */
function spyDb(opts: {
  /** SQL がこの接頭辞で始まる文の run の直前に呼ぶ(『読んでからロックを取るまでの間』の再現)。 */
  beforeRun?: { prefix: string; hook: () => Promise<void> };
  /** bind の引数の個数を記録する。 */
  binds?: number[];
  /** batch に渡した文の数を記録する。 */
  batches?: number[];
}): D1Database {
  const real = testDb();
  return new Proxy(real, {
    get(target, prop) {
      if (prop === "prepare") {
        return (sql: string) => {
          const st = target.prepare(sql);
          const origBind = st.bind.bind(st);
          st.bind = (...args: unknown[]) => {
            opts.binds?.push(args.length);
            const b = origBind(...args);
            const br = opts.beforeRun;
            if (br && sql.startsWith(br.prefix)) {
              const origRun = b.run.bind(b);
              b.run = (async () => {
                await br.hook();
                return origRun();
              }) as typeof b.run;
            }
            return b;
          };
          return st;
        };
      }
      if (prop === "batch") {
        return (stmts: D1PreparedStatement[]) => {
          opts.batches?.push(stmts.length);
          return target.batch(stmts);
        };
      }
      const v = Reflect.get(target, prop);
      return typeof v === "function" ? v.bind(target) : v;
    },
  });
}

describe("ロックを失った保持者は進捗も対応も書かない(stale を奪われた場合)", () => {
  async function stealDuring(
    thief: { next_page: number | null; completed_at: string | null },
    clockMs: () => number,
  ) {
    const db = testDb();
    await db
      .prepare(
        `UPDATE document_scan_state SET lock_owner = 'thief', lock_started_at = ?, next_page = ?, started_at = 'S', completed_at = ?
         WHERE document_kind = 'estimate'`,
      )
      .bind(new Date(clockMs()).toISOString(), thief.next_page, thief.completed_at)
      .run();
  }

  it("奪われたあと、元の保持者は進捗を巻き戻せず、対応も書けない。その書類は scan_in_progress で縮退する", async () => {
    const { index, clock } = setup();
    const api = mockOutboundFetch({
      [projectsKey]: pagedHandler(
        () => projects(9),
        async () => {
          await stealDuring({ next_page: 7, completed_at: null }, clock.now);
          return undefined;
        },
      ),
    });
    const r = await index.lookup("estimate", 1005);
    expect(r).toEqual({ status: "unresolved", reason: "scan_in_progress" });
    expect(pagesOf(api.calls)).toEqual([1]); // 以後の走査を中止する
    const st = await testDb()
      .prepare("SELECT * FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ next_page: number; started_at: string; lock_owner: string }>();
    expect(st).toMatchObject({ next_page: 7, started_at: "S", lock_owner: "thief" });
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(0);
  });

  it("奪った側の完了印(completed_at)を、元の保持者が消せない", async () => {
    const { index, clock } = setup();
    const doneAt = "2026-11-05T00:00:05Z";
    mockOutboundFetch({
      [projectsKey]: pagedHandler(
        () => projects(9),
        async () => {
          await stealDuring({ next_page: null, completed_at: doneAt }, clock.now);
          return undefined;
        },
      ),
    });
    const r = await index.lookup("estimate", 1005);
    expect(r).toEqual({ status: "unresolved", reason: "scan_in_progress" });
    const st = await testDb()
      .prepare(
        "SELECT next_page, completed_at, lock_owner FROM document_scan_state WHERE document_kind = 'estimate'",
      )
      .first<{ next_page: number | null; completed_at: string | null; lock_owner: string }>();
    expect(st).toEqual({ next_page: null, completed_at: doneAt, lock_owner: "thief" });
  });
});

describe("ロックを取ったあとに走査の状態を読み直す", () => {
  it("状態を読んでからロックを取るまでの間に別の呼び出しが走査を完了していたら、走査せず not_found", async () => {
    const clock = fakeClock();
    const db = spyDb({
      beforeRun: {
        prefix: "INSERT OR IGNORE INTO document_scan_state",
        hook: async () => {
          await testDb()
            .prepare(
              `INSERT OR REPLACE INTO document_scan_state (document_kind, next_page, started_at, completed_at, query_version) VALUES ('estimate', NULL, 'S', ?, ${CACHE_QUERY_VERSION})`,
            )
            .bind(new Date(clock.now()).toISOString())
            .run();
        },
      },
    });
    const index = createDocumentIndex({
      db,
      client: testClient(clock.now),
      now: clock.now,
      perPage: PER_PAGE,
    });
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    expect(await index.lookup("estimate", 99999)).toEqual({ status: "miss", reason: "not_found" });
    expect(api.calls).toHaveLength(0);
    const st = await testDb()
      .prepare("SELECT lock_owner FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ lock_owner: string | null }>();
    expect(st?.lock_owner).toBeNull();
  });

  it("その間に対応表へ入っていたら、走査せずそれを返す", async () => {
    const clock = fakeClock();
    const db = spyDb({
      beforeRun: {
        prefix: "INSERT OR IGNORE INTO document_scan_state",
        hook: async () => {
          await testDb()
            .prepare(
              "INSERT OR REPLACE INTO document_project_map (document_kind, document_id, project_id, recorded_at) VALUES ('estimate', 1005, 5, 'x')",
            )
            .run();
        },
      },
    });
    const index = createDocumentIndex({
      db,
      client: testClient(clock.now),
      now: clock.now,
      perPage: PER_PAGE,
    });
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    expect(await index.lookup("estimate", 1005)).toEqual({ status: "hit", projectId: 5 });
    expect(api.calls).toHaveLength(0);
  });

  it("その間に別の呼び出しが進捗を進めていたら、読み直した次のページから続ける", async () => {
    const clock = fakeClock();
    const db = spyDb({
      beforeRun: {
        prefix: "INSERT OR IGNORE INTO document_scan_state",
        hook: async () => {
          await testDb()
            .prepare(
              `INSERT OR REPLACE INTO document_scan_state (document_kind, next_page, started_at, query_version) VALUES ('estimate', 3, 'S', ${CACHE_QUERY_VERSION})`,
            )
            .run();
        },
      },
    });
    const index = createDocumentIndex({
      db,
      client: testClient(clock.now),
      now: clock.now,
      perPage: PER_PAGE,
    });
    const api = mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(9)) });
    await index.lookup("estimate", 1009);
    expect(pagesOf(api.calls)[0]).toBe(3);
  });
});

describe("対応表への記録は複数行にまとめる(D1 のクエリ数とバインド数)", () => {
  it("1 ページ(100 案件 = 200 対応)の走査の batch は、文の数を抑え、1 文のバインドは 100 以下", async () => {
    const clock = fakeClock();
    const binds: number[] = [];
    const batches: number[] = [];
    const db = spyDb({ binds, batches });
    const index = createDocumentIndex({
      db,
      client: testClient(clock.now),
      now: clock.now,
      perPage: 100,
    });
    mockOutboundFetch({ [projectsKey]: pagedHandler(() => projects(100)) });
    await index.lookup("delivery", 2001);
    // 200 対応 → 1 文 24 行として 9 文 + 進捗 1 文。1 行 1 文(200 文)ではない。
    const pageBatch = batches.find((n) => n > 1);
    expect(pageBatch).toBeLessThanOrEqual(12);
    expect(Math.max(...binds)).toBeLessThanOrEqual(100);
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(200);
  });

  it("record も同様にまとめる。重複する書類 ID があっても全件入る", async () => {
    const binds: number[] = [];
    const batches: number[] = [];
    const index = createDocumentIndex({
      db: spyDb({ binds, batches }),
      client: testClient(),
      perPage: 100,
    });
    await index.record("delivery", projects(100) as never);
    expect(batches[0]).toBeLessThanOrEqual(9);
    expect(Math.max(...binds)).toBeLessThanOrEqual(100);
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(200);
  });
});

describe("1 回の呼び出しあたりのページ上限", () => {
  it("上限のページ数を取ったら、未解決(deadline)で止める。進捗は保存され、ロックは解放される", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({
      [projectsKey]: pagedHandler(() => projects(PER_PAGE * (MAX_SCAN_PAGES_PER_CALL + 5))),
    });
    expect(await index.lookup("estimate", 999999)).toEqual({
      status: "unresolved",
      reason: "deadline",
    });
    expect(api.calls).toHaveLength(MAX_SCAN_PAGES_PER_CALL);
    expect(pagesOf(api.calls)).toEqual(
      Array.from({ length: MAX_SCAN_PAGES_PER_CALL }, (_, i) => i + 1),
    );
    const st = await testDb()
      .prepare("SELECT * FROM document_scan_state WHERE document_kind = 'estimate'")
      .first<{ next_page: number; completed_at: string | null; lock_owner: string | null }>();
    expect(st?.next_page).toBe(MAX_SCAN_PAGES_PER_CALL + 1);
    expect(st?.completed_at).toBeNull();
    expect(st?.lock_owner).toBeNull();
  });

  it("次の呼び出しは保存した次のページから続け、残りを走査し終えれば not_found になる", async () => {
    const { index } = setup();
    const api = mockOutboundFetch({
      [projectsKey]: pagedHandler(() => projects(PER_PAGE * (MAX_SCAN_PAGES_PER_CALL + 5))),
    });
    await index.lookup("estimate", 999999);
    api.calls.length = 0;
    expect(await index.lookup("estimate", 999999)).toEqual({ status: "miss", reason: "not_found" });
    expect(pagesOf(api.calls)[0]).toBe(MAX_SCAN_PAGES_PER_CALL + 1);
    expect(api.calls.length).toBeLessThanOrEqual(MAX_SCAN_PAGES_PER_CALL);
  });

  it("上限ちょうどのページで見つかれば hit(上限のために見逃さない)", async () => {
    const { index } = setup();
    mockOutboundFetch({
      [projectsKey]: pagedHandler(() => projects(PER_PAGE * (MAX_SCAN_PAGES_PER_CALL + 5))),
    });
    const lastPageProject = PER_PAGE * MAX_SCAN_PAGES_PER_CALL;
    expect(await index.lookup("estimate", 1000 + lastPageProject)).toEqual({
      status: "hit",
      projectId: lastPageProject,
    });
  });
});

describe("上流の ID の確認(正の安全な整数だけを使う)", () => {
  const BAD = [0, -1, 1.5, Number.NaN, 2 ** 53, "7", null];

  it.each(Object.values(DOCUMENT_KIND_INFO))(
    "$kind: 書類 ID が不正なものは取り出さない",
    (info) => {
      const doc = (id: unknown) => ({ id });
      const p = {
        id: 1,
        estimate: doc(10),
        order: doc(10),
        deliveries: [doc(10), ...BAD.map(doc)],
        receipts: [doc(10), ...BAD.map(doc)],
      };
      const bad = {
        id: 1,
        estimate: doc(BAD[1]),
        order: doc(BAD[2]),
      };
      expect(info.ids(p as never)).toEqual([10]);
      expect(info.ids(bad as never)).toEqual([]);
    },
  );

  it("案件 ID が不正な案件の書類は、対応表に覚えない", async () => {
    const { index } = setup();
    mockOutboundFetch({});
    await index.record("estimate", [
      ...BAD.map((id, i) => ({ id, estimate: { id: 500 + i } })),
      { id: 3, estimate: { id: 600 } },
    ] as never);
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(1);
    expect(await index.lookup("estimate", 600)).toEqual({ status: "hit", projectId: 3 });
  });

  it("走査で見つけた案件 ID が不正なら hit にしない", async () => {
    const { index } = setup();
    mockOutboundFetch({
      [projectsKey]: pagedHandler(() => [{ id: -5, estimate: { id: 777 }, deliveries: [] }]),
    });
    const r = await index.lookup("estimate", 777);
    expect(r.status).not.toBe("hit");
    expect(await count("SELECT count(*) AS c FROM document_project_map")).toBe(0);
  });
});
