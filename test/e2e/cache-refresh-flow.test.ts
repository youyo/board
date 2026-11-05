import { env } from "cloudflare:workers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createBoardClientFromEnv } from "../../src/board/client";
import { cacheStatus } from "../../src/cache/admin";
import { RefreshInProgressError, toErrorEnvelope } from "../../src/errors";
import { createRepositories } from "../../src/repository";
import { BASE, makeEntities, pagedHandler } from "../helpers/board-list";
import { fakeClock, resetCache } from "../helpers/d1";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

// 実物の D1(miniflare)と本物の BOARD クライアント(createBoardClientFromEnv)を使い、BOARD API は outbound fetch のモックにする。
// MCP ツールはまだない(M04・M05)ので、repository の公開関数を直接呼ぶ。
// 置き場所が test/e2e/ なのは、キャッシュ → refresh → repository → クライアント → (モックした)HTTP を通しで確かめる
// テストだから(個々の部品の単体は test/unit/)。

const KEY = `GET ${BASE}/v1/clients`;
const hang = () => new Promise<never>(() => {});

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache(env.DB);
});

function setup() {
  const clock = fakeClock();
  const client = createBoardClientFromEnv(env, {
    now: clock.now,
    maxRetries: 0,
    sleep: async () => {},
  });
  const repos = createRepositories({ db: env.DB, client, now: clock.now, pageBudgetFloorMs: 0 });
  return { clock, repos };
}

describe("E2E: full refresh を複数回の呼び出しに分けて完了させる", () => {
  it("締め切りごとに partial → 再開 → complete。途中も参照でき、完了で古い世代が消え、cache の鮮度情報が進む", async () => {
    const { clock, repos } = setup();
    const data = makeEntities(450);
    // 事前のキャッシュ(旧世代)。のちに API から消える 9999 を含む。
    const seedApi = mockOutboundFetch({
      [KEY]: pagedHandler(() => [
        ...makeEntities(3),
        { id: 9999, name: "消えるクライアント", updated_at: "2026-01-01T00:00:00.000+09:00" },
      ]),
    });
    await repos.clients.list({}, { refresh: "full" });
    expect(seedApi.calls).toHaveLength(1);
    vi.restoreAllMocks();
    clock.advance(86_400_000);
    const day2 = clock.now();

    // 1 ページ 4 秒かかる BOARD。1 回の呼び出しの持ち時間は 10 秒 = 2 ページまで。
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(
        () => data,
        () => {
          clock.advance(4_000);
          return undefined;
        },
      ),
    });
    const call = () =>
      repos.clients.list({}, { refresh: "full" }, { deadline: clock.now() + 10_000 });
    const pageOf = (i: number) => new URL(api.calls[i]?.url ?? "").searchParams.get("page");

    const r1 = await call();
    expect(r1.cache[0]).toMatchObject({ refresh_state: "partial" });
    // 旧世代の最初の full 完了の記録は残っている(新しい full は未完了)。
    expect(r1.cache[0]?.full_refreshed_at).toBe("2026-11-05T00:00:00Z");
    // 新旧の世代の和集合: 旧世代の 9999 が見え、新しい 200 件も入っている。
    expect(r1.items.some((e) => e.id === 9999)).toBe(true);
    expect(r1.items).toHaveLength(200 + 1); // id 1-3 は新世代で上書き、9999 は旧世代のまま

    const r2 = await call();
    expect(r2.cache[0]?.refresh_state).toBe("partial");
    expect(r2.items.some((e) => e.id === 9999)).toBe(true);
    expect(r2.items).toHaveLength(400 + 1);

    const r3 = await call();
    expect(r3.cache[0]).toMatchObject({
      refresh_state: "complete",
    });
    expect(r3.cache[0]?.full_refreshed_at).not.toBe("2026-11-05T00:00:00Z");
    expect(r3.items).toHaveLength(450);
    expect(r3.items.some((e) => e.id === 9999)).toBe(false);
    expect(new Date(r3.cache[0]?.full_refreshed_at ?? "").getTime()).toBeGreaterThan(day2);

    // 取りに行ったページは 1,2 / 3,4 / 5 の順(再開しても重複して取らない)。
    expect(api.calls.map((_, i) => pageOf(i))).toEqual(["1", "2", "3", "4", "5"]);

    // 状態の確認(運用向け)。
    const row = (await cacheStatus(env.DB, { now: clock.now })).find(
      (r) => r.resource === "clients",
    );
    expect(row).toMatchObject({ cached_rows: 450, refreshing: false });
    expect(row?.full_in_progress).toBeUndefined();
  });
});

describe("E2E: delta refresh のあとに参照する", () => {
  it("差分が反映され、キャッシュ側のフィルタ(fold)でも新しい名前で引ける。参照だけなら API は呼ばない", async () => {
    const { clock, repos } = setup();
    const data = makeEntities(5, (i) => ({ name: `会社${i + 1}` }));
    const api = mockOutboundFetch({ [KEY]: pagedHandler(() => data) });
    await repos.clients.list({}, { refresh: "full" });
    const afterFull = api.calls.length;

    clock.advance(3_600_000);
    // 3 番の改名と 6 番の新規。delta はカーソル以降だけが返るモック。
    const changed = [
      { id: 3, name: "ＮＥＷ商事", updated_at: "2026-05-01T09:00:00.000+09:00" },
      { id: 6, name: "新規会社", updated_at: "2026-05-02T09:00:00.000+09:00" },
    ];
    vi.restoreAllMocks();
    const deltaApi = mockOutboundFetch({
      [KEY]: pagedHandler(() => changed),
    });
    const r = await repos.clients.list({}, { refresh: "delta" });
    expect(r.items).toHaveLength(6);
    expect(r.cache[0]).toMatchObject({
      cached_at: "2026-11-05T01:00:00Z",
      full_refreshed_at: "2026-11-05T00:00:00Z", // delta は full の記録を動かさない
      refresh_state: "complete",
    });
    // 差分の取得には、full の最後のカーソル(会社 5 番の updated_at)が updated_at_gteq として渡った。
    expect(new URL(deltaApi.calls[0]?.url ?? "").searchParams.get("updated_at_gteq")).toBe(
      "2026-01-01 00:00:04",
    );

    const found = await repos.clients.list({ nameCont: "new商事" }, { refresh: "none" });
    expect(found.items.map((e) => e.id)).toEqual([3]);
    expect(found.source).toBe("cache");
    expect(deltaApi.calls).toHaveLength(1); // 参照では BOARD を呼んでいない
    expect(afterFull).toBe(1);

    // 次の delta のカーソルは取得分の最大値に進んでいる。
    vi.restoreAllMocks();
    const next = mockOutboundFetch({ [KEY]: pagedHandler(() => []) });
    await repos.clients.list({}, { refresh: "delta" });
    expect(new URL(next.calls[0]?.url ?? "").searchParams.get("updated_at_gteq")).toBe(
      "2026-05-02 09:00:00",
    );
  });
});

describe("E2E: 2 つの refresh を同時に走らせると、一方は refresh_in_progress", () => {
  it("後から来たほうは待たずに失敗し(retry_after_seconds 30)、先のほうは最後まで完了する。終われば再試行が通る", async () => {
    const { repos } = setup();
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let entered!: () => void;
    const inHandler = new Promise<void>((resolve) => {
      entered = resolve;
    });
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(3),
        async () => {
          entered();
          await gate;
          return undefined;
        },
      ),
    });

    const first = repos.clients.list({}, { refresh: "full" });
    await inHandler; // 1 つ目が BOARD への問い合わせ中(ロックを保持している)
    const second = await repos.clients.list({}, { refresh: "delta" }).catch((e) => e);

    expect(second).toBeInstanceOf(RefreshInProgressError);
    expect(toErrorEnvelope(second)).toEqual({
      schema_version: "1",
      error: {
        code: "refresh_in_progress",
        message: expect.stringContaining("clients"),
        retryable: true,
        retry_after_seconds: 30,
      },
    });
    expect(api.calls).toHaveLength(1); // 2 つ目は BOARD に届いていない

    release();
    const done = await first;
    expect(done.items).toHaveLength(3);
    expect(done.cache[0]?.refresh_state).toBe("complete");

    const retry = await repos.clients.list({}, { refresh: "delta" });
    expect(retry.cache[0]?.refresh_state).toBe("complete");
  });

  it("別のリソースの refresh は互いに邪魔しない", async () => {
    const { repos } = setup();
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(2),
        async () => {
          await gate;
          return undefined;
        },
      ),
      [`GET ${BASE}/v1/users`]: pagedHandler(() => [{ id: 1, name: "u", email: "u@example.com" }]),
    });
    const a = repos.clients.list({}, { refresh: "full" });
    // クライアントは直列なので users は clients の問い合わせの後ろに並ぶが、ロックでは弾かれない。
    const b = repos.users.list({}, { refresh: "full" });
    release();
    const [ra, rb] = await Promise.all([a, b]);
    expect(ra.items).toHaveLength(2);
    expect(rb.items).toHaveLength(1);
  });
});

describe("E2E: キャッシュが空のときの挙動(Go の repository 層の実態)", () => {
  it("cache-first: ゼロフィルタは空(API を呼ばない)/ 絞り込みは API にフォールバック / get は API → 保存", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({
      [KEY]: pagedHandler(() => [{ id: 1, name: "ライブ" }]),
      [`GET ${BASE}/v1/clients/1`]: () => jsonResponse({ id: 1, name: "ライブ" }),
    });
    const empty = await repos.clients.list({});
    expect(empty).toMatchObject({ items: [], source: "cache", cache: [{ resource: "clients" }] });
    expect(empty.cache[0]?.cached_at).toBeUndefined();
    expect(api.calls).toHaveLength(0);

    const live = await repos.clients.list({ nameCont: "ライ" });
    expect(live).toMatchObject({ source: "api", items: [{ id: 1 }] });
    expect(api.calls).toHaveLength(1);

    expect((await repos.clients.get(1)).source).toBe("api");
    expect((await repos.clients.get(1)).source).toBe("cache");
  });

  it("nonzero-api: ゼロフィルタは空 / 非ゼロは常に API", async () => {
    const { repos } = setup();
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/groups`]: pagedHandler(() => [{ id: 1, name: "g" }]),
    });
    expect((await repos.groups.list({})).items).toEqual([]);
    expect(api.calls).toHaveLength(0);
    expect((await repos.groups.list({ nameCont: "g" })).source).toBe("api");
  });
});

describe("E2E: 秘密情報はキャッシュにもエラーにも残らない", () => {
  it("D1 の全テーブルに API キー・トークンが入らない。失敗した refresh のエラー記録にも本文は載らない", async () => {
    const { repos } = setup();
    mockOutboundFetch({
      [KEY]: pagedHandler(
        () => makeEntities(250),
        (page) =>
          page === 2
            ? jsonResponse(
                { message: `token ${env.BOARD_API_TOKEN} key ${env.BOARD_API_KEY} rejected` },
                500,
              )
            : undefined,
      ),
    });
    const err = await repos.clients.list({}, { refresh: "full" }).catch((e) => e);
    expect(String(err.message)).not.toContain(env.BOARD_API_TOKEN);
    expect(String(err.message)).not.toContain(env.BOARD_API_KEY);

    const dump = JSON.stringify([
      (await env.DB.prepare("SELECT * FROM sync_state").all()).results,
      (await env.DB.prepare("SELECT * FROM cache_meta").all()).results,
      (await env.DB.prepare("SELECT resource_name, payload_json FROM resource_cache").all())
        .results,
    ]);
    expect(dump).not.toContain(env.BOARD_API_TOKEN);
    expect(dump).not.toContain(env.BOARD_API_KEY);
    expect(dump).toContain("last_error_code");
  });
});

describe("E2E: document の get", () => {
  it("API → 保存 → 次はキャッシュ。refresh=full で取り直す", async () => {
    const { repos } = setup();
    let total = "100";
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/documents/orders/*`]: () => jsonResponse({ id: 5, total, details: [] }),
    });
    expect((await repos.orders.get(5)).source).toBe("api");
    expect((await repos.orders.get(5)).source).toBe("cache");
    total = "250";
    const fresh = await repos.orders.get(5, { refresh: "full" });
    expect(fresh.item).toMatchObject({ total: "250" });
    expect(api.calls).toHaveLength(2);
    void hang;
  });
});
