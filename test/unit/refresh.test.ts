import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BoardApiError } from "../../src/board/errors";
import { buildClientsQuery } from "../../src/board/resources/clients";
import { countEntries, getEntry, listEntries } from "../../src/cache/resource-cache";
import { getSyncState } from "../../src/cache/sync-state";
import { RefreshInProgressError } from "../../src/errors";
import { LOCK_STALE_MS, tryAcquireLock } from "../../src/refresh/lock";
import {
  cursorAfterFull,
  FULL_RESUME_WINDOW_MS,
  isoToBoardDateTime,
  type RefreshTarget,
  refreshResource,
} from "../../src/refresh/refresher";
import { BASE, makeEntities, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

const clients: RefreshTarget = {
  resource: "clients",
  path: "/v1/clients",
  buildQuery: buildClientsQuery,
};
const KEY = `GET ${BASE}/v1/clients`;
const hang = () => new Promise<never>(() => {});

afterEach(() => {
  vi.restoreAllMocks();
});

beforeEach(async () => {
  await resetCache();
});

function setup(start?: number) {
  const clock = fakeClock(start);
  const db = testDb();
  const deps = { db, client: testClient(clock.now), now: clock.now, pageBudgetFloorMs: 0 };
  return { clock, db, deps };
}

describe("isoToBoardDateTime(Go の isoToBoardDateTime。元のオフセットの時刻部分をそのまま使う)", () => {
  it.each([
    ["", ""],
    ["2024-12-26T10:14:11.000+09:00", "2024-12-26 10:14:11"],
    ["2024-12-26T10:14:11+09:00", "2024-12-26 10:14:11"],
    ["2024-12-26T10:14:11Z", "2024-12-26 10:14:11"],
    ["2024-12-26T10:14:11.123456789Z", "2024-12-26 10:14:11"],
    ["2024-12-26 10:14:11", "2024-12-26 10:14:11"],
    ["2024-12-26T10:14:11-05:30", "2024-12-26 10:14:11"],
    // 解釈できない値はそのまま返す(サーバーに明示的に拒否させる)。
    ["yesterday", "yesterday"],
  ])("%s → %s", (input, want) => {
    expect(isoToBoardDateTime(input)).toBe(want);
  });
});

describe("cursorAfterFull(full の完了後の delta カーソル)", () => {
  it("見た updated_at の最大値が full の開始より十分古ければ、その最大値", () => {
    expect(cursorAfterFull("2026-01-01T00:00:59.000+09:00", "2026-11-05T00:00:00Z")).toBe(
      "2026-01-01T00:00:59.000+09:00",
    );
  });

  it("最大値が full の開始(の 5 分前)より新しければ、開始の 5 分前を同じオフセットで表す(full 中の更新を取りこぼさない)", () => {
    // 開始 2026-11-05T00:00:00Z の 5 分前 = 23:55:00Z = JST 08:55:00
    expect(cursorAfterFull("2026-11-05T09:30:00.000+09:00", "2026-11-05T00:00:00Z")).toBe(
      "2026-11-05T08:55:00.000+09:00",
    );
  });

  it("見たものがなければ null(カーソルなし)", () => {
    expect(cursorAfterFull(null, "2026-11-05T00:00:00Z")).toBeNull();
  });
});

describe("カーソルの元になる updated_at は形式が正しいものだけを使う", () => {
  const GARBAGE = [
    "zzzz-not-a-date",
    "9999 garbage",
    "2099-99",
    "' OR 1=1 --",
    "2026-13-45T25:61:61Z0",
  ];

  it("full: 解釈できない値(辞書順で大きい)は最大値に使わない", async () => {
    const { db, deps } = setup();
    const data = [
      { id: 1, name: "a", updated_at: "2026-02-01T00:00:00.000+09:00" },
      ...GARBAGE.map((g, i) => ({ id: 10 + i, name: "g", updated_at: g })),
    ];
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await refreshResource(deps, clients, "full");
    expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBe(
      "2026-02-01T00:00:00.000+09:00",
    );
  });

  it("delta: 解釈できない値はカーソルにも gteq にも入らない", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        { id: 1, name: "a", updated_at: "2026-02-01T00:00:00.000+09:00" },
      ]),
    });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();

    clock.advance(3_600_000);
    mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        { id: 2, name: "b", updated_at: "2026-03-01T10:00:00.000+09:00" },
        ...GARBAGE.map((g, i) => ({ id: 20 + i, name: "g", updated_at: g })),
      ]),
    });
    const out = await refreshResource(deps, clients, "delta");
    expect(out.state).toBe("complete");
    expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBe(
      "2026-03-01T10:00:00.000+09:00",
    );
  });

  it("形式が正しい値がひとつもなければ、カーソルは動かない(null のまま)", async () => {
    const { db, deps } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => GARBAGE.map((g, i) => ({ id: 1 + i, name: "g", updated_at: g }))),
    });
    await refreshResource(deps, clients, "full");
    expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBeNull();
  });
});

describe("full refresh", () => {
  it("全ページを取って complete にし、世代 1 で保存して full_refreshed_at と cursor を記録する", async () => {
    const { db, deps, clock } = setup();
    const data = makeEntities(250);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });

    const out = await refreshResource(deps, clients, "full");

    expect(out).toMatchObject({
      resource: "clients",
      mode: "full",
      state: "complete",
      fetched: 250,
      pages: 3,
    });
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["1", "2", "3"]);
    expect(api.calls[0]?.url).toContain("per_page=100");
    expect(await countEntries(db, "clients")).toBe(250);
    const st = await getSyncState(db, "clients");
    expect(st).toMatchObject({
      last_synced_at: "2026-11-05T00:00:00Z",
      last_full_synced_at: "2026-11-05T00:00:00Z",
      last_sync_mode: "full",
      last_sync_status: "success",
      full_generation: 1,
      full_next_page: null,
      consecutive_failures: 0,
      refresh_owner: null,
    });
    // 見た最大値(2026-01-01...)は開始より古いので、そのままカーソルになる。
    expect(st?.cursor_updated_at).toBe("2026-01-01T00:00:59.000+09:00");
    expect((await getEntry(db, "clients", 250))?.generation).toBe(1);
    expect(clock.now()).toBe(Date.parse("2026-11-05T00:00:00Z"));
  });

  it("ちょうど per_page の倍数でも、空の次ページを取って完了する", async () => {
    const { deps } = setup();
    const data = makeEntities(200);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    const out = await refreshResource(deps, clients, "full");
    expect(out).toMatchObject({ state: "complete", fetched: 200, pages: 3 });
    expect(api.calls).toHaveLength(3);
  });

  it("2 回目の full は世代が進み、API から消えたエンティティを削除して更新を反映する", async () => {
    const { db, deps, clock } = setup();
    let data = makeEntities(5);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await refreshResource(deps, clients, "full");
    expect(await countEntries(db, "clients")).toBe(5);

    clock.advance(86_400_000);
    data = makeEntities(5, (i) => ({ name: `改名${i + 1}` })).filter((e) => e.id !== 3);
    await refreshResource(deps, clients, "full");

    const rows = await listEntries<{ id: number; name: string }>(db, "clients");
    expect(rows.map((r) => r.entity.id).sort()).toEqual([1, 2, 4, 5]);
    expect((await getEntry<{ name: string }>(db, "clients", 1))?.entity.name).toBe("改名1");
    expect((await getSyncState(db, "clients"))?.full_generation).toBe(2);
  });

  it("残り時間がページ 1 枚分の目安を下回ったら、進捗を保存して partial で止まり、ロックを解放する", async () => {
    const { db, deps, clock } = setup();
    const data = makeEntities(450);
    // 1 ページ 4 秒かかる。締め切り 10 秒: 1 枚目の後に残り 6 秒(=4 秒 x 1.5 に届く)、2 枚目の後に残り 2 秒で止まる。
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(4_000);
          return undefined;
        },
      ),
    });
    const out = await refreshResource(deps, clients, "full", { deadline: clock.now() + 10_000 });

    expect(out).toMatchObject({
      state: "partial",
      mode: "full",
      fetched: 200,
      pages: 2,
      nextPage: 3,
    });
    expect(await countEntries(db, "clients")).toBe(200);
    const st = await getSyncState(db, "clients");
    expect(st).toMatchObject({
      full_next_page: 3,
      full_generation: 1,
      last_full_synced_at: null,
      last_synced_at: null, // partial は『同期した』ことにしない
      refresh_owner: null,
    });
  });

  it("BOARD クライアントが締め切りのエラーを返したときも partial(待っていたページから再開できる)", async () => {
    // 実時間: 2 ページ目が返らないまま締め切りが来る。
    const db = testDb();
    const deps = { db, client: testClient(), pageBudgetFloorMs: 0 };
    const data = makeEntities(250);
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        (page) => (page === 2 ? hang() : undefined),
      ),
    });
    const out = await refreshResource(deps, clients, "full", { deadline: Date.now() + 200 });
    expect(out).toMatchObject({ state: "partial", fetched: 100, pages: 1, nextPage: 2 });
    expect((await getSyncState(db, "clients"))?.full_next_page).toBe(2);
    expect((await getSyncState(db, "clients"))?.refresh_owner).toBeNull();
    expect(await countEntries(db, "clients")).toBe(100);
  });

  it("次の full は最後に取り終えたページから(1 ページ重ねて)再開し、最後まで取ったら古い世代を消して complete にする", async () => {
    const { db, deps, clock } = setup();
    // 既存のキャッシュ(世代 1)に、あとで API から消えるエンティティ 999 がある。
    let data = makeEntities(250);
    mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        ...data,
        { id: 999, name: "消える", updated_at: "2026-01-01T00:00:00.000+09:00" },
      ]),
    });
    await refreshResource(deps, clients, "full");
    expect(await getEntry(db, "clients", 999)).not.toBeNull();
    vi.restoreAllMocks();

    // 世代 2 の full を 1 ページだけで止める。
    clock.advance(60_000);
    data = makeEntities(250, (i) => ({ name: `v2-${i + 1}` }));
    let allowSlow = true;
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          if (allowSlow) clock.advance(9_000);
          return undefined;
        },
      ),
    });
    const p1 = await refreshResource(deps, clients, "full", { deadline: clock.now() + 10_000 });
    expect(p1).toMatchObject({ state: "partial", nextPage: 2 });
    // 途中でも古い行(999 を含む)は残り、参照できる(新旧の世代の和集合)。
    expect(await getEntry(db, "clients", 999)).not.toBeNull();
    expect((await getEntry<{ name: string }>(db, "clients", 1))?.entity.name).toBe("v2-1");
    expect((await getEntry<{ name: string }>(db, "clients", 150))?.entity.name).toBe("顧客150");
    vi.restoreAllMocks();

    // 再開: 最後に取り終えたページ(1)から取り直す(1 ページ重ねる)。
    allowSlow = false;
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    clock.advance(30_000);
    const p2 = await refreshResource(deps, clients, "full");
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["1", "2", "3"]);
    expect(p2).toMatchObject({ state: "complete", fetched: 250, pages: 3 });
    expect(await getEntry(db, "clients", 999)).toBeNull(); // 古い世代は消える
    expect((await getEntry<{ name: string }>(db, "clients", 150))?.entity.name).toBe("v2-150");
    expect(await countEntries(db, "clients")).toBe(250);
    const st = await getSyncState(db, "clients");
    expect(st).toMatchObject({
      full_generation: 2,
      full_next_page: null,
      last_full_synced_at: "2026-11-05T00:01:39Z",
    });
  });

  it("再開できる期間(1 時間)を過ぎた partial は、新しい世代で最初のページからやり直す", async () => {
    const { db, deps, clock } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(9_000);
          return undefined;
        },
      ),
    });
    await refreshResource(deps, clients, "full", { deadline: clock.now() + 10_000 });
    expect((await getSyncState(db, "clients"))?.full_next_page).toBe(2);
    vi.restoreAllMocks();

    clock.advance(FULL_RESUME_WINDOW_MS + 1_000);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    const out = await refreshResource(deps, clients, "full");
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["1", "2", "3"]);
    expect(out).toMatchObject({ state: "complete", pages: 3 });
    expect((await getSyncState(db, "clients"))?.full_generation).toBe(2);
  });

  it("full の途中に入った delta の書き込みは進行中の世代を付け、full の完了時に消されない", async () => {
    const { db, deps, clock } = setup();
    // 完了済みの full(世代 1)でカーソルを作る。
    let data = makeEntities(150);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();

    // 世代 2 の full を 1 ページで止める。
    clock.advance(60_000);
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(9_000);
          return undefined;
        },
      ),
    });
    await refreshResource(deps, clients, "full", { deadline: clock.now() + 10_000 });
    vi.restoreAllMocks();

    // delta が、full がまだ通っていない行 140 の更新を取り込む(世代 2 が付く)。
    mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        { id: 140, name: "delta で更新", updated_at: "2026-06-01T00:00:00.000+09:00" },
      ]),
    });
    const d = await refreshResource(deps, clients, "delta");
    expect(d).toMatchObject({ mode: "delta", state: "complete", fetched: 1 });
    expect((await getEntry(db, "clients", 140))?.generation).toBe(2);
    vi.restoreAllMocks();

    // full を再開して完了。この full のデータには 140 がない(ページのずれの想定)が、delta で入れた行は残る。
    data = makeEntities(150).filter((e) => e.id !== 140);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    const done = await refreshResource(deps, clients, "full");
    expect(done.state).toBe("complete");
    expect((await getEntry<{ name: string }>(db, "clients", 140))?.entity.name).toBe(
      "delta で更新",
    );
  });

  it("API が失敗しても、それまでのページは残り、失敗が記録され、例外が伝わり、ロックは解放される", async () => {
    const { db, deps } = setup();
    const data = makeEntities(250);
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        (page) => (page === 2 ? jsonResponse({ message: "boom" }, 500) : undefined),
      ),
    });
    const e = await refreshResource(deps, clients, "full").catch((x) => x);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(await countEntries(db, "clients")).toBe(100);
    expect(await getSyncState(db, "clients")).toMatchObject({
      full_next_page: 2,
      last_sync_status: "error",
      last_error_code: "api_error",
      consecutive_failures: 1,
      refresh_owner: null,
    });
  });

  it("ロックが保持されていれば、BOARD を呼ばずに RefreshInProgressError", async () => {
    const { db, deps, clock } = setup();
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "other" });
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(1)) });
    await expect(refreshResource(deps, clients, "full")).rejects.toBeInstanceOf(
      RefreshInProgressError,
    );
    expect(api.calls).toHaveLength(0);
    expect(await countEntries(db, "clients")).toBe(0);
  });

  it("id のないエンティティが混じっていたら api_error(その回の書き込みは入らない)で、ロックは解放される", async () => {
    const { db, deps } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => [{ id: 1, name: "a" }, { name: "no id" }]) });
    const e = await refreshResource(deps, clients, "full").catch((x) => x);
    expect(e).toMatchObject({ code: "api_error" });
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await getSyncState(db, "clients")).toMatchObject({
      refresh_owner: null,
      last_error_code: "api_error",
      consecutive_failures: 1,
    });
  });

  it("応答が配列でなければ api_error", async () => {
    const { deps } = setup();
    mockOutboundFetch({ [KEY]: () => jsonResponse({ not: "array" }) });
    await expect(refreshResource(deps, clients, "full")).rejects.toMatchObject({
      code: "api_error",
    });
  });
});

describe("delta refresh", () => {
  async function seedCompleted(deps: ReturnType<typeof setup>["deps"], n = 5) {
    const data = makeEntities(n);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();
  }

  it("カーソルを updated_at_gteq(YYYY-MM-DD HH:MM:SS)で渡して差分を upsert し、カーソルを最大値に進める", async () => {
    const { db, deps, clock } = setup();
    await seedCompleted(deps);
    expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBe(
      "2026-01-01T00:00:04.000+09:00",
    );

    clock.advance(3_600_000);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        { id: 2, name: "更新", updated_at: "2026-03-01T10:00:00.000+09:00" },
        { id: 77, name: "新規", updated_at: "2026-03-02T11:22:33.000+09:00" },
      ]),
    });
    const out = await refreshResource(deps, clients, "delta");

    expect(out).toMatchObject({ mode: "delta", state: "complete", fetched: 2, pages: 1 });
    expect(new URL(api.calls[0]?.url ?? "").searchParams.get("updated_at_gteq")).toBe(
      "2026-01-01 00:00:04",
    );
    expect((await getEntry<{ name: string }>(db, "clients", 2))?.entity.name).toBe("更新");
    expect(await getEntry(db, "clients", 77)).not.toBeNull();
    expect(await countEntries(db, "clients")).toBe(6);
    expect(await getSyncState(db, "clients")).toMatchObject({
      cursor_updated_at: "2026-03-02T11:22:33.000+09:00",
      last_sync_mode: "delta",
      last_sync_status: "success",
      last_synced_at: "2026-11-05T01:00:00Z",
      last_full_synced_at: "2026-11-05T00:00:00Z", // delta は full_refreshed_at を動かさない
      refresh_owner: null,
    });
  });

  it("差分がなければカーソルは変わらず、last_synced_at だけ進む", async () => {
    const { db, deps, clock } = setup();
    await seedCompleted(deps);
    const before = (await getSyncState(db, "clients"))?.cursor_updated_at;
    clock.advance(1_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => []) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({ state: "complete", fetched: 0 });
    const st = await getSyncState(db, "clients");
    expect(st?.cursor_updated_at).toBe(before);
    expect(st?.last_synced_at).toBe("2026-11-05T00:00:01Z");
  });

  it("差分が複数ページにわたっても全部取る", async () => {
    const { db, deps } = setup();
    await seedCompleted(deps);
    const diff = Array.from({ length: 230 }, (_, i) => ({
      id: 1000 + i,
      name: `d${i}`,
      updated_at: "2026-05-01T00:00:00.000+09:00",
    }));
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => diff) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({ state: "complete", fetched: 230, pages: 3 });
    expect(api.calls).toHaveLength(3);
    expect(await countEntries(db, "clients")).toBe(235);
  });

  it("カーソルがなければ delta は(再開できる)full として動く: updated_at_gteq を付けない", async () => {
    const { db, deps } = setup();
    const data = makeEntities(3);
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({ mode: "full", state: "complete", fetched: 3 });
    expect(new URL(api.calls[0]?.url ?? "").searchParams.has("updated_at_gteq")).toBe(false);
    expect((await getSyncState(db, "clients"))?.last_full_synced_at).not.toBeNull();
  });

  it("API の失敗は failed(古いキャッシュはそのまま、カーソルも動かない)。例外にはしない", async () => {
    const { db, deps } = setup();
    await seedCompleted(deps);
    const before = await getSyncState(db, "clients");
    mockOutboundFetch({ [KEY]: () => jsonResponse({ message: "nope" }, 401) });
    const out = await refreshResource(deps, clients, "delta");
    expect(out).toMatchObject({
      mode: "delta",
      state: "failed",
      errorCode: "authentication_error",
    });
    const st = await getSyncState(db, "clients");
    expect(st?.cursor_updated_at).toBe(before?.cursor_updated_at);
    expect(st).toMatchObject({
      last_sync_status: "error",
      last_error_code: "authentication_error",
      consecutive_failures: 1,
      refresh_owner: null,
    });
    expect(await countEntries(db, "clients")).toBe(5);
  });

  it("締め切りで途中までしか取れなかったら partial: 取れた分は入れるがカーソルは動かさない", async () => {
    const { db, deps } = setup();
    await seedCompleted(deps);
    const before = (await getSyncState(db, "clients"))?.cursor_updated_at;
    const diff = Array.from({ length: 250 }, (_, i) => ({
      id: 1000 + i,
      name: `d${i}`,
      updated_at: "2026-05-01T00:00:00.000+09:00",
    }));
    // こちらは実時間で動かす(クライアントの締め切りも実時間)。
    const realDeps = { db, client: testClient(), pageBudgetFloorMs: 0 };
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => diff,
        (page) => (page === 2 ? hang() : undefined),
      ),
    });
    const out = await refreshResource(realDeps, clients, "delta", { deadline: Date.now() + 200 });
    expect(out).toMatchObject({ mode: "delta", state: "partial", fetched: 100 });
    expect(await countEntries(db, "clients")).toBe(105);
    expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBe(before);
  });

  describe("再開(full と同じ方式)", () => {
    /** 差分 n 件。先頭ページ(item 0)の updated_at が最大で、後ろほど古い。 */
    const diffOf = (n: number, base = "2026-05-01T12:00:00") =>
      Array.from({ length: n }, (_, i) => ({
        id: 1000 + i,
        name: `d${i}`,
        updated_at: `${base}.${String(999 - i).padStart(3, "0")}+09:00`,
      }));

    /** 1 ページ 4 秒かかり、締め切り 10 秒: 2 ページ取って partial になる。 */
    function slowPages(clock: ReturnType<typeof fakeClock>, rows: () => unknown[]) {
      return mockOutboundFetch({
        [KEY]: pagedHandler(rows, () => {
          clock.advance(4_000);
          return undefined;
        }),
      });
    }
    const pagesOf = (api: { calls: { url: string }[] }) =>
      api.calls.map((c) => Number(new URL(c.url).searchParams.get("page")));

    it("partial になったら進捗(次のページ・開始時刻・見た updated_at の最大値)を保存し、カーソルは動かさない", async () => {
      const { db, deps, clock } = setup();
      await seedCompleted(deps);
      const before = await getSyncState(db, "clients");
      clock.advance(60_000);
      const startMs = clock.now();
      const diff = diffOf(450);
      slowPages(clock, () => diff);
      const out = await refreshResource(deps, clients, "delta", { deadline: clock.now() + 10_000 });
      expect(out).toMatchObject({ mode: "delta", state: "partial", fetched: 200, pages: 2 });
      expect(await getSyncState(db, "clients")).toMatchObject({
        cursor_updated_at: before?.cursor_updated_at,
        delta_next_page: 3,
        delta_started_at: new Date(startMs).toISOString().replace(/\.\d{3}Z$/, "Z"),
        delta_max_updated_at: diff[0]?.updated_at,
        refresh_owner: null,
      });
    });

    it("次の delta は最後に取り終えたページから(1 ページ重ねて)取り直し、最後まで取ったらカーソルを進めて進捗を消す", async () => {
      const { db, deps, clock } = setup();
      await seedCompleted(deps);
      clock.advance(60_000);
      const diff = diffOf(450);
      slowPages(clock, () => diff);
      await refreshResource(deps, clients, "delta", { deadline: clock.now() + 10_000 });
      vi.restoreAllMocks();

      clock.advance(1_000);
      const api = mockOutboundFetch({ [KEY]: pagedHandler(() => diff) });
      const out = await refreshResource(deps, clients, "delta");
      // next_page=3 の 1 つ前(2)から重ねて取り直す。
      expect(pagesOf(api)).toEqual([2, 3, 4, 5]);
      expect(new URL(api.calls[0]?.url ?? "").searchParams.get("updated_at_gteq")).toBe(
        "2026-01-01 00:00:04",
      );
      expect(out).toMatchObject({ mode: "delta", state: "complete" });
      expect(await countEntries(db, "clients")).toBe(5 + 450);
      // 最大値は最初の回(ページ 1)で見たもの。保存した進捗が引き継がれる。
      expect(await getSyncState(db, "clients")).toMatchObject({
        cursor_updated_at: diff[0]?.updated_at,
        delta_next_page: null,
        delta_started_at: null,
        delta_max_updated_at: null,
        last_sync_status: "success",
      });
    });

    it("カーソルは min(見た最大値, delta の開始の 5 分前)。最大値が開始より新しければ開始の 5 分前になる", async () => {
      const { db, deps, clock } = setup();
      await seedCompleted(deps);
      clock.advance(60_000); // 2026-11-05T00:01:00Z = 09:01:00+09:00
      // 開始の直前に更新された行(最大値が開始より新しい)。
      mockOutboundFetch({
        [KEY]: pagedHandler(() => [
          { id: 900, name: "最近", updated_at: "2026-11-05T09:00:30.000+09:00" },
        ]),
      });
      await refreshResource(deps, clients, "delta");
      expect((await getSyncState(db, "clients"))?.cursor_updated_at).toBe(
        "2026-11-05T08:56:00.000+09:00",
      );
    });

    it("再開できる期間(1 時間)を過ぎた進捗は捨てて、最初のページからやり直す", async () => {
      const { deps, clock } = setup();
      await seedCompleted(deps);
      clock.advance(60_000);
      const diff = diffOf(450);
      slowPages(clock, () => diff);
      await refreshResource(deps, clients, "delta", { deadline: clock.now() + 10_000 });
      vi.restoreAllMocks();

      clock.advance(FULL_RESUME_WINDOW_MS + 1_000);
      const api = mockOutboundFetch({ [KEY]: pagedHandler(() => diff) });
      const out = await refreshResource(deps, clients, "delta");
      expect(pagesOf(api)).toEqual([1, 2, 3, 4, 5]);
      expect(out).toMatchObject({ state: "complete" });
    });

    it("API の失敗(failed)でも、それまでのページの進捗は残り、次の delta はそこから再開する", async () => {
      const { db, deps, clock } = setup();
      await seedCompleted(deps);
      clock.advance(60_000);
      const diff = diffOf(450);
      mockOutboundFetch({
        [KEY]: pagedHandler(
          () => diff,
          (page) => (page === 3 ? jsonResponse({ message: "boom" }, 401) : undefined),
        ),
      });
      const out = await refreshResource(deps, clients, "delta");
      expect(out).toMatchObject({ state: "failed" });
      expect((await getSyncState(db, "clients"))?.delta_next_page).toBe(3);
      vi.restoreAllMocks();

      const api = mockOutboundFetch({ [KEY]: pagedHandler(() => diff) });
      await refreshResource(deps, clients, "delta");
      expect(pagesOf(api)).toEqual([2, 3, 4, 5]);
    });
  });

  it("ロックが保持されていれば RefreshInProgressError(delta も待たない)", async () => {
    const { db, deps, clock } = setup();
    await seedCompleted(deps);
    await tryAcquireLock(db, "clients", { now: clock.now, owner: "other" });
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => []) });
    await expect(refreshResource(deps, clients, "delta")).rejects.toBeInstanceOf(
      RefreshInProgressError,
    );
    expect(api.calls).toHaveLength(0);
  });
});

describe("呼び出し元の中断", () => {
  it("signal が abort されたら失敗を記録せずに例外を伝え、ロックは解放する", async () => {
    const { db, deps } = setup();
    const ac = new AbortController();
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(250),
        () => {
          ac.abort();
          return undefined;
        },
      ),
    });
    await expect(
      refreshResource(deps, clients, "full", { signal: ac.signal }),
    ).rejects.toMatchObject({
      code: "api_error",
    });
    const st = await getSyncState(db, "clients");
    expect(st?.consecutive_failures).toBe(0);
    expect(st?.refresh_owner).toBeNull();
  });
});

describe("full の再開でページがずれても、実在する行を消さない", () => {
  it("再開は最後に取り終えたページから取り直す: 合間に前のページの行が消えても、繰り上がった行(Z)が残る", async () => {
    const { db, deps, clock } = setup();
    let data = makeEntities(250);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data, undefined, { totalHeader: true }) });
    await refreshResource(deps, clients, "full"); // 旧世代: 1..250
    vi.restoreAllMocks();

    // 世代 2 の full を 1 ページ(1..100)で止める。Z = id 101 は 2 ページ目の先頭。
    clock.advance(60_000);
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(9_000);
          return undefined;
        },
        { totalHeader: true },
      ),
    });
    const p1 = await refreshResource(deps, clients, "full", { deadline: clock.now() + 10_000 });
    expect(p1).toMatchObject({ state: "partial", nextPage: 2 });
    vi.restoreAllMocks();

    // 合間に BOARD で id 5 が削除され、Z(101)が 1 ページ目の末尾に繰り上がる。
    // あわせて、まだ取っていない末尾の id 250 も削除される(これは旧世代として消えるのが正しい)。
    data = data.filter((e) => e.id !== 5 && e.id !== 250);
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => data, undefined, { totalHeader: true }),
    });
    const p2 = await refreshResource(deps, clients, "full");
    expect(api.calls.map((c) => new URL(c.url).searchParams.get("page"))).toEqual(["1", "2", "3"]);
    expect(p2).toMatchObject({ state: "complete" });
    expect(await getEntry(db, "clients", 101)).not.toBeNull(); // Z は残る
    expect(await getEntry(db, "clients", 250)).toBeNull(); // 実際に消えた旧世代の行は消える
    // id 5 は削除前のページ取得で新世代に入っているため、次の full まで残る(既知の限界。計画書に記載)。
    expect(await countEntries(db, "clients")).toBe(249);
    expect((await getSyncState(db, "clients"))?.full_prune_skipped).toBe(0);
  });

  it("新世代で取れた件数が総件数(X-Total-Count)に満たなければ、旧世代を消さず complete にして full_prune_skipped を記録する。次の full で消える", async () => {
    const { db, deps, clock } = setup();
    const data = makeEntities(300);
    mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await refreshResource(deps, clients, "full"); // 旧世代: 1..300
    vi.restoreAllMocks();

    // 世代 2: 150 件しか返らないが、総件数のヘッダは 160 件と言っている(取りこぼしが疑われる)。
    clock.advance(60_000);
    const short = data.slice(0, 150);
    mockOutboundFetch({ [KEY]: pagedHandler(() => short, undefined, { totalHeader: 160 }) });
    const out = await refreshResource(deps, clients, "full");
    expect(out).toMatchObject({ state: "complete", pruneSkipped: true });
    expect(await countEntries(db, "clients")).toBe(300); // 旧世代の 151..300 を残した
    expect(await getSyncState(db, "clients")).toMatchObject({
      full_next_page: null,
      full_prune_skipped: 1,
      last_sync_status: "success",
    });
    vi.restoreAllMocks();

    // 次の full: 件数が総件数に届けば、旧世代(世代 2 に更新されなかった行)を消し、フラグを戻す。
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => short, undefined, { totalHeader: true }) });
    const again = await refreshResource(deps, clients, "full");
    expect(again.pruneSkipped).toBeFalsy();
    expect(await countEntries(db, "clients")).toBe(150);
    expect((await getSyncState(db, "clients"))?.full_prune_skipped).toBe(0);
  });

  it("総件数のヘッダがなければ、従来どおり旧世代を消す", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(300)) });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();
    clock.advance(60_000);
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(150)) });
    await refreshResource(deps, clients, "full");
    expect(await countEntries(db, "clients")).toBe(150);
  });
});

describe("stale ロックを奪われた元の保持者は、その後 sync_state にも行にも書き込めない", () => {
  /** A の 1 ページ目の取得中に 11 分が経ち、B が stale ロックを奪う。 */
  function stolenDuringFetch(db: D1Database, clock: ReturnType<typeof fakeClock>) {
    return async () => {
      clock.advance(LOCK_STALE_MS + 1_000);
      await tryAcquireLock(db, "clients", { now: clock.now, owner: "B" });
      return undefined;
    };
  }

  it("full: 書き込みは反映されず、refresh_in_progress で失敗する。B のロックと進捗は壊れない", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(250), stolenDuringFetch(db, clock)),
    });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "full", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await getSyncState(db, "clients")).toMatchObject({
      refresh_owner: "B",
      full_next_page: 1, // A は進めていない
      full_max_updated_at: null,
      last_sync_status: "partial",
      consecutive_failures: 0, // A の失敗を B の状態に書かない
    });
  });

  it("full の最終ページ: 旧世代の削除もカーソルの更新も反映されない", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(5)) });
    await refreshResource(deps, clients, "full"); // 旧世代 1..5
    vi.restoreAllMocks();
    const before = await getSyncState(db, "clients");

    clock.advance(60_000);
    mockOutboundFetch({
      [KEY]: pagedHandler(() => makeEntities(3), stolenDuringFetch(db, clock)),
    });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "full", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    expect(await countEntries(db, "clients")).toBe(5); // 旧世代は消えていない
    const st = await getSyncState(db, "clients");
    expect(st?.cursor_updated_at).toBe(before?.cursor_updated_at);
    expect(st?.last_full_synced_at).toBe(before?.last_full_synced_at);
    expect(st?.refresh_owner).toBe("B");
  });

  it("delta: 書き込みは反映されず、failed ではなく refresh_in_progress で失敗する", async () => {
    const { db, deps, clock } = setup();
    mockOutboundFetch({ [KEY]: pagedHandler(() => makeEntities(5)) });
    await refreshResource(deps, clients, "full");
    vi.restoreAllMocks();
    const before = await getSyncState(db, "clients");

    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => [{ id: 99, name: "新規", updated_at: "2026-06-01T00:00:00.000+09:00" }],
        stolenDuringFetch(db, clock),
      ),
    });
    const e = await refreshResource({ ...deps, owner: "A" }, clients, "delta", {
      deadline: clock.now() + 3_600_000,
    }).catch((x) => x);
    expect(e).toBeInstanceOf(RefreshInProgressError);
    expect(await getEntry(db, "clients", 99)).toBeNull();
    const st = await getSyncState(db, "clients");
    expect(st?.cursor_updated_at).toBe(before?.cursor_updated_at);
    expect(st?.consecutive_failures).toBe(0);
    expect(st?.refresh_owner).toBe("B");
  });
});
