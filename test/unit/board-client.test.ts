import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type BoardClient,
  type BoardClientOptions,
  clearBoardClientCache,
  createBoardClient,
  createBoardClientFromEnv,
  DEFAULT_CALL_BUDGET_MS,
  DEFAULT_MAX_RETRIES,
  DEFAULT_TIMEOUT_MS,
  retryDelayMs,
} from "../../src/board/client";
import { BoardApiError, isDeadlineError } from "../../src/board/errors";
import { getClient, listClients } from "../../src/board/resources/clients";
import { BoardError, toErrorEnvelope } from "../../src/errors";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

const BASE = "https://api.board.test";
const KEY = "KEY-secret-0123456789"; // gitleaks:allow テスト用のダミー値（実在しない）
const TOKEN = "TOKEN-secret-9876543210";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 待機を記録するだけで実際には待たない。jitter は固定。 */
function makeClient(over: Partial<BoardClientOptions> = {}) {
  const waits: number[] = [];
  const client = createBoardClient({
    baseUrl: BASE,
    apiKey: KEY,
    apiToken: TOKEN,
    sleep: async (ms) => {
      waits.push(ms);
    },
    random: () => 0.5,
    ...over,
  });
  return { client, waits };
}

/** Go の http.Client で言う「ハンドラー」。呼ばれるたびにキューから応答を返す。 */
function sequence(...responses: Array<Response | (() => Response) | Error>) {
  let i = 0;
  return () => {
    const r = responses[Math.min(i++, responses.length - 1)] as Response | (() => Response) | Error;
    if (r instanceof Error) throw r;
    return typeof r === "function" ? r() : r.clone();
  };
}

describe("認証とURL (Go の auth.go / NewRequest)", () => {
  it("x-api-key と Bearer を付け、GET で BASE + パス + クエリを呼ぶ", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse([]),
    });
    const { client } = makeClient();
    await client.get("/v1/clients", "page=1&per_page=100");
    expect(m.calls).toHaveLength(1);
    const c = m.calls[0];
    expect(c?.method).toBe("GET");
    expect(c?.url).toBe(`${BASE}/v1/clients?page=1&per_page=100`);
    expect(c?.headers.get("x-api-key")).toBe(KEY);
    expect(c?.headers.get("authorization")).toBe(`Bearer ${TOKEN}`);
    expect(c?.headers.get("accept")).toBe("application/json");
  });

  it("baseUrl の末尾のスラッシュは除く。クエリが空なら ? を付けない", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/users/1": () => jsonResponse({}),
    });
    const { client } = makeClient({ baseUrl: `${BASE}///` });
    await client.get("/v1/users/1");
    expect(m.calls[0]?.url).toBe(`${BASE}/v1/users/1`);
  });

  it("createBoardClientFromEnv は env の 3 値を使う", async () => {
    const m = mockOutboundFetch({ "GET https://api.board.test/v1/groups": () => jsonResponse([]) });
    const client = createBoardClientFromEnv({
      BOARD_BASE_URL: BASE,
      BOARD_API_KEY: KEY,
      BOARD_API_TOKEN: TOKEN,
    });
    await client.get("/v1/groups");
    expect(m.calls[0]?.headers.get("x-api-key")).toBe(KEY);
  });

  it("createBoardClientFromEnv は前後の空白・改行を除いた値を使う(redact も効く)", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/groups": () =>
        new Response(JSON.stringify({ message: `bad ${KEY} ${TOKEN}` }), { status: 400 }),
    });
    const client = createBoardClientFromEnv({
      BOARD_BASE_URL: `${BASE}\n`,
      BOARD_API_KEY: `${KEY}\n`,
      BOARD_API_TOKEN: ` ${TOKEN}\n`,
    });
    const e = await client.get("/v1/groups").catch((x) => x);
    expect(m.calls[0]?.headers.get("x-api-key")).toBe(KEY);
    expect(m.calls[0]?.headers.get("authorization")).toBe(`Bearer ${TOKEN}`);
    expect(m.calls[0]?.url).toBe(`${BASE}/v1/groups`);
    expect(String(e.message)).not.toContain(KEY);
    expect(String(e.message)).not.toContain(TOKEN);
  });

  it("成功時は本文(JSON)とヘッダーを返す", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients/1": () =>
        new Response(JSON.stringify({ id: 1, unknown_field: { a: [1] } }), {
          headers: { "Content-Type": "application/json", "X-Total-Count": "9" },
        }),
    });
    const { client } = makeClient();
    const res = await client.get("/v1/clients/1");
    expect(res.body).toEqual({ id: 1, unknown_field: { a: [1] } });
    expect(res.headers.get("x-total-count")).toBe("9");
  });

  it("成功したのに JSON でない・空の本文は api_error(再試行しない)", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response("<html>oops</html>", { status: 200 }),
    });
    const { client, waits } = makeClient();
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(e).toBeInstanceOf(BoardError);
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(false);
    expect(m.calls).toHaveLength(1);
    expect(waits).toEqual([]);
  });
});

describe("エラー変換 (Go の parseError)", () => {
  it.each([
    [401, "authentication_error"],
    [403, "permission_error"],
    [404, "not_found"],
    [422, "argument_error"],
    [400, "argument_error"],
  ] as const)("HTTP %i は再試行せず %s", async (status, code) => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients/1": () => jsonResponse({ message: "nope" }, status),
    });
    const { client, waits } = makeClient();
    const e = await client.get("/v1/clients/1").catch((x) => x);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.status).toBe(status);
    expect(toErrorEnvelope(e).error).toEqual({
      code,
      message: `BOARD API error (HTTP ${status}): nope`,
      retryable: false,
    });
    expect(m.calls).toHaveLength(1);
    expect(waits).toEqual([]);
  });

  it("エラー本文は message、なければ error を使う。JSON でなければステータスだけ", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/a": () => jsonResponse({ error: "bad things" }, 404),
      "GET https://api.board.test/v1/b": () => new Response("not json", { status: 404 }),
      "GET https://api.board.test/v1/c": () => jsonResponse({ message: "m", error: "e" }, 404),
    });
    const { client } = makeClient();
    expect((await client.get("/v1/a").catch((x) => x)).message).toBe(
      "BOARD API error (HTTP 404): bad things",
    );
    expect((await client.get("/v1/b").catch((x) => x)).message).toBe("BOARD API error (HTTP 404).");
    expect((await client.get("/v1/c").catch((x) => x)).message).toBe(
      "BOARD API error (HTTP 404): m",
    );
  });

  it("長い message は 500 文字で切る", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/a": () => jsonResponse({ message: "x".repeat(2000) }, 404),
    });
    const { client } = makeClient();
    const e = await client.get("/v1/a").catch((x) => x);
    expect(e.message.length).toBeLessThan(600);
  });
});

describe("リダイレクト", () => {
  it("追わない(x-api-key を別のオリジンに送らない)。3xx は api_error で再試行しない", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response(null, { status: 302, headers: { Location: "https://evil.example/steal" } }),
    });
    const { client, waits } = makeClient();
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.status).toBe(302);
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(false);
    expect(m.calls).toHaveLength(1);
    expect(waits).toEqual([]);
    expect(m.spy.mock.calls[0]?.[1]?.redirect).toBe("manual");
  });
});

describe("再試行 (Go の retry.go)", () => {
  // 仕様変更(追補): Go の 5 回は CLI 前提。Worker では呼び出し単位の締め切りがあるので 3 回にする。
  it("既定値: 最大 3 回再試行・1 試行 30 秒・呼び出し全体 30 秒", () => {
    expect(DEFAULT_MAX_RETRIES).toBe(3);
    expect(DEFAULT_TIMEOUT_MS).toBe(30_000);
    expect(DEFAULT_CALL_BUDGET_MS).toBe(30_000);
  });

  it.each([429, 500, 502, 503])(
    "HTTP %i は再試行して、成功すればその本文を返す",
    async (status) => {
      const m = mockOutboundFetch({
        "GET https://api.board.test/v1/clients": sequence(
          jsonResponse({ message: "busy" }, status),
          jsonResponse([{ id: 1 }]),
        ),
      });
      const { client, waits } = makeClient();
      const res = await client.get("/v1/clients");
      expect(res.body).toEqual([{ id: 1 }]);
      expect(m.calls).toHaveLength(2);
      expect(waits).toHaveLength(1);
    },
  );

  it("通信失敗も再試行する", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": sequence(
        new TypeError("fetch failed"),
        jsonResponse([]),
      ),
    });
    const { client, waits } = makeClient();
    expect((await client.get("/v1/clients")).body).toEqual([]);
    expect(m.calls).toHaveLength(2);
    expect(waits).toHaveLength(1);
  });

  it("上限まで失敗したら最後のエラーを投げる(試行は 1 + maxRetries 回)", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse({ message: "busy" }, 429),
    });
    const { client, waits } = makeClient();
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.code).toBe("rate_limited");
    expect(e.retryable).toBe(true);
    expect(m.calls).toHaveLength(4);
    expect(waits).toHaveLength(3);
  });

  it("maxRetries=0 は再試行しない。maxRetries=2 なら 3 回", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse({}, 500),
    });
    await makeClient({ maxRetries: 0 })
      .client.get("/v1/clients")
      .catch(() => undefined);
    expect(m.calls).toHaveLength(1);
    await makeClient({ maxRetries: 2 })
      .client.get("/v1/clients")
      .catch(() => undefined);
    expect(m.calls).toHaveLength(4);
  });

  it("Retry-After(整数秒)を優先して待つ", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": sequence(
        new Response("{}", { status: 429, headers: { "Retry-After": "7" } }),
        jsonResponse([]),
      ),
    });
    const { client, waits } = makeClient();
    await client.get("/v1/clients");
    expect(waits).toEqual([7000]);
  });

  it("Retry-After は 5xx でも効く。60 秒で頭打ち", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": sequence(
        new Response("{}", { status: 503, headers: { "Retry-After": "3600" } }),
        jsonResponse([]),
      ),
    });
    const { client, waits } = makeClient();
    // 仕様変更(追補): 既定の締め切りは 30 秒なので、60 秒待つには締め切りを広げる必要がある。
    await client.get("/v1/clients", undefined, { deadline: Date.now() + 120_000 });
    expect(waits).toEqual([60_000]);
  });

  it("Retry-After が 0・小数・日付・不正なら指数バックオフにする", async () => {
    for (const ra of ["0", "1.5", "Wed, 21 Oct 2026 07:28:00 GMT", "abc", ""]) {
      vi.restoreAllMocks();
      mockOutboundFetch({
        "GET https://api.board.test/v1/clients": sequence(
          new Response("{}", { status: 429, headers: { "Retry-After": ra } }),
          jsonResponse([]),
        ),
      });
      const { client, waits } = makeClient();
      await client.get("/v1/clients");
      // attempt 0 → 500ms * 1、random 0.5 → 250ms
      expect(waits, `Retry-After=${JSON.stringify(ra)}`).toEqual([250]);
    }
  });

  it("バックオフは 500ms × 2^n の full jitter(上限 30 秒)", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse({}, 500),
    });
    const { client, waits } = makeClient({ random: () => 0.999999 });
    await client.get("/v1/clients").catch(() => undefined);
    // 500, 1000, 2000 の手前(floor(exp * 0.999999))。再試行は 3 回(仕様変更: 5 → 3)
    expect(waits).toEqual([499, 999, 1999]);
  });

  it("retryDelayMs: 指数・Retry-After 優先・上限 (Go の TestCalcBackoff_*)", () => {
    const zero = () => 0;
    const almost = () => 0.999999;
    expect(retryDelayMs(0, undefined, zero)).toBe(0);
    expect(retryDelayMs(3, undefined, almost)).toBe(3999);
    expect(retryDelayMs(10, undefined, almost)).toBe(29_999); // 30 秒で頭打ち
    expect(retryDelayMs(100, undefined, almost)).toBe(29_999);
    expect(retryDelayMs(0, 5, almost)).toBe(5000);
    expect(retryDelayMs(0, 600, almost)).toBe(60_000);
    expect(retryDelayMs(0, 0, almost)).toBe(499);
  });
});

describe("直列化", () => {
  it("同じクライアントでは同時に 1 リクエストだけ。順序を保つ", async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const order: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = new URL(String((input as Request).url ?? input));
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      order.push(`start ${url.pathname}`);
      await new Promise((r) => setTimeout(r, 15));
      order.push(`end ${url.pathname}`);
      inFlight--;
      return jsonResponse({ path: url.pathname });
    });
    const { client } = makeClient();
    const results = await Promise.all([
      client.get("/v1/a"),
      client.get("/v1/b"),
      client.get("/v1/c"),
    ]);
    expect(maxInFlight).toBe(1);
    expect(order).toEqual([
      "start /v1/a",
      "end /v1/a",
      "start /v1/b",
      "end /v1/b",
      "start /v1/c",
      "end /v1/c",
    ]);
    expect(results.map((r) => (r.body as { path: string }).path)).toEqual([
      "/v1/a",
      "/v1/b",
      "/v1/c",
    ]);
  });

  it("再試行の待機の間も次のリクエストは始めない。先のリクエストが失敗しても後続は流れる", async () => {
    const events: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const path = new URL(String((input as Request).url ?? input)).pathname;
      events.push(`fetch ${path}`);
      return path === "/v1/a" ? jsonResponse({}, 404) : jsonResponse({ ok: true });
    });
    const { client } = makeClient({
      sleep: async () => {
        events.push("sleep");
      },
    });
    const a = client.get("/v1/a").catch((e) => e);
    const b = client.get("/v1/b");
    expect(await a).toBeInstanceOf(BoardApiError);
    expect((await b).body).toEqual({ ok: true });
    expect(events).toEqual(["fetch /v1/a", "fetch /v1/b"]);
  });

  it("429 の再試行の待機が終わるまで次のリクエストは待つ", async () => {
    const events: string[] = [];
    let first = true;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const path = new URL(String((input as Request).url ?? input)).pathname;
      events.push(`fetch ${path}`);
      if (path === "/v1/a" && first) {
        first = false;
        return jsonResponse({}, 429);
      }
      return jsonResponse({});
    });
    const { client } = makeClient({
      sleep: async () => {
        events.push("sleep");
        await new Promise((r) => setTimeout(r, 10));
        events.push("woke");
      },
    });
    await Promise.all([client.get("/v1/a"), client.get("/v1/b")]);
    expect(events).toEqual(["fetch /v1/a", "sleep", "woke", "fetch /v1/a", "fetch /v1/b"]);
  });
});

describe("タイムアウト(本文の読み取りまで含める)", () => {
  it("応答ヘッダーが来ないまま期限が来たら api_error(retryable)。期限ごとに再試行する", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockImplementation(
      (_input, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError")),
          );
        }),
    );
    const { client, waits } = makeClient({ timeoutMs: 20, maxRetries: 1 });
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.status).toBe(0);
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(true);
    expect(e.message).toBe("BOARD API request timed out after 20ms.");
    expect(spy).toHaveBeenCalledTimes(2);
    expect(waits).toHaveLength(1);
  });

  it("ヘッダーは来たが本文が途中で止まった場合も期限で失敗する", async () => {
    let cancelled = false;
    vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(new TextEncoder().encode('[{"id":'));
          // close しない = 本文が止まる
        },
        cancel() {
          cancelled = true;
        },
      });
      return new Response(body, { status: 200, headers: { "Content-Type": "application/json" } });
    });
    const { client } = makeClient({ timeoutMs: 20, maxRetries: 0 });
    const started = Date.now();
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(Date.now() - started).toBeLessThan(2000);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.message).toBe("BOARD API request timed out after 20ms.");
    expect(cancelled).toBe(true);
  });

  it("エラー応答の本文が止まっても期限で終わり、ステータスだけでエラーにする", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(new TextEncoder().encode('{"message":'));
        },
      });
      return new Response(body, { status: 404 });
    });
    const { client } = makeClient({ timeoutMs: 20, maxRetries: 0 });
    const e = await client.get("/v1/clients/1").catch((x) => x);
    expect(e.status).toBe(404);
    expect(e.code).toBe("not_found");
    expect(e.message).toBe("BOARD API error (HTTP 404).");
  });
});

describe("締め切りとキャンセル(呼び出しコンテキスト)", () => {
  const never = () => new Promise<never>(() => {});
  const tick = (ms = 10) => new Promise((r) => setTimeout(r, ms));
  const expectDeadlineError = (e: unknown, code: string) => {
    expect(e).toBeInstanceOf(BoardError);
    expect((e as BoardError).code).toBe(code);
    expect((e as BoardError).retryable).toBe(true);
    expect((e as BoardError).message).toMatch(/deadline/);
    // 呼び出し側(refresh)が文面に頼らず『締め切り』を見分けられる。
    expect(isDeadlineError(e)).toBe(true);
  };
  const expectAbortError = (e: unknown) => {
    expect(isDeadlineError(e)).toBe(false);
    expect(e).toBeInstanceOf(BoardError);
    expect((e as BoardError).code).toBe("api_error");
    expect((e as BoardError).retryable).toBe(true);
    expect((e as BoardError).message).toMatch(/aborted/);
  };
  /** signal が abort されたら reject する fetch。 */
  const hangingFetch = () =>
    vi.spyOn(globalThis, "fetch").mockImplementation(
      (_input, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError")),
          );
        }),
    );

  it("Retry-After が残り時間を超えるなら待たずに rate_limited(retryable)で失敗する", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response("{}", { status: 429, headers: { "Retry-After": "10" } }),
    });
    const sleeps: number[] = [];
    const { client } = makeClient({
      sleep: async (ms) => {
        sleeps.push(ms);
      },
    });
    const e = await client
      .get("/v1/clients", undefined, { deadline: Date.now() + 5_000 })
      .catch((x) => x);
    expectDeadlineError(e, "rate_limited");
    expect(sleeps).toEqual([]);
    expect(m.calls).toHaveLength(1);
  });

  it("5xx・通信失敗でバックオフが残り時間を超えるなら api_error(retryable)で即失敗する", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response("{}", { status: 503, headers: { "Retry-After": "10" } }),
    });
    const { client, waits } = makeClient();
    const e = await client
      .get("/v1/clients", undefined, { deadline: Date.now() + 5_000 })
      .catch((x) => x);
    expectDeadlineError(e, "api_error");
    expect(waits).toEqual([]);
  });

  it("締め切りに収まる Retry-After なら待って成功する", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": sequence(
        new Response("{}", { status: 429, headers: { "Retry-After": "2" } }),
        jsonResponse([{ id: 1 }]),
      ),
    });
    const { client, waits } = makeClient();
    const res = await client.get("/v1/clients", undefined, { deadline: Date.now() + 5_000 });
    expect(res.body).toEqual([{ id: 1 }]);
    expect(waits).toEqual([2000]);
    expect(m.calls).toHaveLength(2);
  });

  it("deadline を省くと呼び出し開始から 30 秒が締め切り(Retry-After 40 秒は待たない)", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response("{}", { status: 429, headers: { "Retry-After": "40" } }),
    });
    const { client, waits } = makeClient();
    const e = await client.get("/v1/clients").catch((x) => x);
    expectDeadlineError(e, "rate_limited");
    expect(waits).toEqual([]);
  });

  it("1 試行の期限は min(timeoutMs, 残り時間)", async () => {
    hangingFetch();
    const { client } = makeClient({ maxRetries: 0 });
    const started = Date.now();
    const e = await client
      .get("/v1/clients", undefined, { deadline: Date.now() + 60 })
      .catch((x) => x);
    expect(Date.now() - started).toBeLessThan(2000);
    expect(e).toBeInstanceOf(BoardApiError);
    expect(e.retryable).toBe(true);
  });

  it("一覧のページ巡回は同じ締め切りを共有する(2 ページ目で期限切れ)", async () => {
    let t = 1_000;
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => {
        t += 20_000; // 1 ページ目に 20 秒かかった
        return jsonResponse(Array.from({ length: 100 }, (_, i) => ({ id: i + 1 })));
      },
    });
    const { client } = makeClient({ now: () => t });
    const e = await listClients(client, {}, undefined, { deadline: 1_000 + 10_000 }).catch(
      (x) => x,
    );
    expectDeadlineError(e, "api_error");
    expect(m.calls).toHaveLength(1);
  });

  it("ctx を省いた一覧は、全ページで 1 つの既定の締め切りを共有する", async () => {
    let t = 0;
    const seen: Array<number | undefined> = [];
    const fake: BoardClient = {
      redact: (x) => x,
      async get(_path, _query, ctx) {
        seen.push(ctx?.deadline);
        t += 1;
        return { body: Array.from({ length: 100 }, () => ({})), headers: new Headers() };
      },
    };
    await listClients(fake, {}, { maxPages: 3 });
    expect(seen).toHaveLength(3);
    expect(new Set(seen).size).toBe(1);
    expect(typeof seen[0]).toBe("number");
    expect(t).toBe(3);
  });

  it("get 系の関数も ctx を client.get に渡す", async () => {
    const ctxs: unknown[] = [];
    const fake: BoardClient = {
      redact: (x) => x,
      async get(_path, _query, ctx) {
        ctxs.push(ctx);
        return { body: { id: 1 }, headers: new Headers() };
      },
    };
    const ctx = { deadline: 123 };
    await getClient(fake, 1, ctx);
    expect(ctxs).toEqual([ctx]);
  });

  it("signal の abort で fetch 中の呼び出しが即座に中断する(再試行しない)", async () => {
    const spy = hangingFetch();
    const { client, waits } = makeClient();
    const ac = new AbortController();
    const p = client.get("/v1/clients", undefined, { signal: ac.signal }).catch((x) => x);
    await tick();
    ac.abort();
    expectAbortError(await p);
    expect(spy).toHaveBeenCalledTimes(1);
    expect(waits).toEqual([]);
  });

  it("すでに abort 済みの signal では fetch を呼ばない", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse([]),
    });
    const { client } = makeClient();
    const e = await client
      .get("/v1/clients", undefined, { signal: AbortSignal.abort() })
      .catch((x) => x);
    expectAbortError(e);
    expect(m.calls).toHaveLength(0);
  });

  it("signal の abort で sleep 中の呼び出しが即座に中断する(sleep に signal が渡る)", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse({}, 500),
    });
    let received: AbortSignal | undefined;
    const { client } = makeClient({
      sleep: (_ms, signal) => {
        received = signal;
        return never(); // signal を無視する sleep でも中断できる
      },
    });
    const ac = new AbortController();
    const p = client.get("/v1/clients", undefined, { signal: ac.signal }).catch((x) => x);
    await tick();
    expect(received).toBeDefined();
    ac.abort();
    expectAbortError(await p);
    expect(m.calls).toHaveLength(1);
  });

  it("signal の abort で直列化キューの待ちが即座に中断する(後続は流れる)", async () => {
    const resolvers: Array<(r: Response) => void> = [];
    const paths: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
      paths.push(new URL(String((input as Request).url ?? input)).pathname);
      return new Promise<Response>((resolve) => resolvers.push(resolve));
    });
    const { client } = makeClient();
    const a = client.get("/v1/a");
    await tick();
    const ac = new AbortController();
    const b = client.get("/v1/b", undefined, { signal: ac.signal }).catch((x) => x);
    const c = client.get("/v1/c");
    await tick();
    ac.abort();
    expectAbortError(await b); // a がまだ終わっていないのに b は返る
    expect(paths).toEqual(["/v1/a"]);
    resolvers[0]?.(jsonResponse({ a: 1 }));
    await a;
    await tick();
    resolvers[1]?.(jsonResponse({ c: 1 }));
    await c;
    expect(paths).toEqual(["/v1/a", "/v1/c"]); // b は BOARD に送られない
  });

  it("キューで待つ間に締め切りを過ぎたら、BOARD に送らずに失敗する", async () => {
    const resolvers: Array<(r: Response) => void> = [];
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(() => new Promise<Response>((resolve) => resolvers.push(resolve)));
    const { client } = makeClient({ now: Date.now });
    const a = client.get("/v1/a");
    await tick();
    const started = Date.now();
    const e = await client.get("/v1/b", undefined, { deadline: Date.now() + 40 }).catch((x) => x);
    expect(Date.now() - started).toBeLessThan(2000);
    expectDeadlineError(e, "api_error");
    expect(spy).toHaveBeenCalledTimes(1);
    resolvers[0]?.(jsonResponse({}));
    await a;
    expect(spy).toHaveBeenCalledTimes(1);
  });
});

describe("createBoardClientFromEnv の共有(isolate 内のキャッシュ)", () => {
  const env = { BOARD_BASE_URL: BASE, BOARD_API_KEY: KEY, BOARD_API_TOKEN: TOKEN };
  afterEach(() => clearBoardClientCache());

  it("同じ設定なら同じインスタンス、設定が違えば別のインスタンス", () => {
    clearBoardClientCache();
    const a = createBoardClientFromEnv(env);
    expect(createBoardClientFromEnv({ ...env })).toBe(a);
    expect(createBoardClientFromEnv({ ...env, BOARD_API_KEY: "other-key" })).not.toBe(a);
    expect(createBoardClientFromEnv({ ...env, BOARD_API_TOKEN: "other-token" })).not.toBe(a);
    expect(createBoardClientFromEnv({ ...env, BOARD_BASE_URL: "https://other.test" })).not.toBe(a);
  });

  it("clearBoardClientCache の後は新しいインスタンス", () => {
    const a = createBoardClientFromEnv(env);
    clearBoardClientCache();
    expect(createBoardClientFromEnv(env)).not.toBe(a);
  });

  it("overrides を渡したときはキャッシュしない", () => {
    const a = createBoardClientFromEnv(env);
    expect(createBoardClientFromEnv(env, { maxRetries: 0 })).not.toBe(a);
    expect(createBoardClientFromEnv(env)).toBe(a);
  });

  it("リクエストをまたいでも直列化が効く", async () => {
    let inFlight = 0;
    let max = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      inFlight++;
      max = Math.max(max, inFlight);
      await new Promise((r) => setTimeout(r, 10));
      inFlight--;
      return jsonResponse({});
    });
    await Promise.all([
      createBoardClientFromEnv(env).get("/v1/a"),
      createBoardClientFromEnv(env).get("/v1/b"),
    ]);
    expect(max).toBe(1);
  });
});

describe("応答本文の上限", () => {
  it("上限を超えたら api_error(再試行しない)", async () => {
    const m = mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => new Response(`["${"a".repeat(200)}"]`),
    });
    const { client } = makeClient({ maxResponseBytes: 100 });
    const e = await client.get("/v1/clients").catch((x) => x);
    expect(e).toBeInstanceOf(BoardError);
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(false);
    expect(m.calls).toHaveLength(1);
  });
});

describe("秘密情報を出さない", () => {
  const secrets = [KEY, TOKEN];
  const expectClean = (e: unknown) => {
    const dump = JSON.stringify([
      (e as Error).message,
      (e as Error).name,
      String(e),
      toErrorEnvelope(e),
      Object.entries(e as object),
      (e as { cause?: unknown }).cause === undefined ? "" : "HAS CAUSE",
      (e as Error).stack?.split("\n")[0],
    ]);
    for (const s of secrets) expect(dump).not.toContain(s);
    expect(dump).not.toContain("HAS CAUSE");
  };

  it("BOARD が本文でキー・トークンを返しても伏せる", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        jsonResponse({ message: `invalid key ${KEY} / token ${TOKEN}` }, 401),
    });
    const { client } = makeClient();
    const e = await client.get("/v1/clients").catch((x) => x);
    expectClean(e);
    expect(e.message).toContain("[REDACTED]");
    expect(e.code).toBe("authentication_error");
  });

  it("fetch の例外の文面にキー・トークンがあっても、原因を捨てて固定文にする", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => {
        throw new Error(`connect failed: x-api-key=${KEY} Authorization: Bearer ${TOKEN}`);
      },
    });
    const { client } = makeClient({ maxRetries: 0 });
    const e = await client.get("/v1/clients").catch((x) => x);
    expectClean(e);
    expect(e.message).toBe("BOARD API is unreachable.");
  });

  it("締め切り・中断のエラーにもキー・トークンを出さない", async () => {
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () =>
        new Response(JSON.stringify({ message: `${KEY} ${TOKEN}` }), {
          status: 429,
          headers: { "Retry-After": "10" },
        }),
    });
    const { client } = makeClient();
    const d = await client
      .get("/v1/clients", undefined, { deadline: Date.now() + 1_000 })
      .catch((x) => x);
    expect(d.message).toMatch(/deadline/);
    expectClean(d);
    const a = await client
      .get("/v1/clients", undefined, { signal: AbortSignal.abort() })
      .catch((x) => x);
    expect(a.message).toMatch(/aborted/);
    expectClean(a);
  });

  it("再試行を使い切っても、ログ(console)にもキー・トークンを出さない", async () => {
    const logs: unknown[][] = [];
    for (const k of ["log", "info", "warn", "error", "debug"] as const) {
      vi.spyOn(console, k).mockImplementation((...a: unknown[]) => {
        logs.push(a);
      });
    }
    mockOutboundFetch({
      "GET https://api.board.test/v1/clients": () => jsonResponse({ message: TOKEN }, 500),
    });
    const { client } = makeClient({ maxRetries: 2 });
    const e = await client.get("/v1/clients").catch((x) => x);
    expectClean(e);
    expect(JSON.stringify(logs)).not.toContain(KEY);
    expect(JSON.stringify(logs)).not.toContain(TOKEN);
  });

  it("redact は文字列中のキー・トークンを伏せる。空の秘密では何もしない", () => {
    const { client } = makeClient();
    expect(client.redact(`a ${KEY} b ${TOKEN} c`)).toBe("a [REDACTED] b [REDACTED] c");
    const empty = createBoardClient({ baseUrl: BASE, apiKey: "", apiToken: "" });
    expect(empty.redact("abc")).toBe("abc");
  });

  it("redact は encodeURIComponent した形のキー・トークンも伏せる", () => {
    const key = "k/ey+1=a b&c";
    const token = "t:ok@en#2%";
    const c = createBoardClient({ baseUrl: BASE, apiKey: key, apiToken: token });
    const text = `url ?k=${encodeURIComponent(key)}&t=${encodeURIComponent(token)} raw ${key} ${token}`;
    const out = c.redact(text);
    expect(out).not.toContain(encodeURIComponent(key));
    expect(out).not.toContain(encodeURIComponent(token));
    expect(out).not.toContain(key);
    expect(out).not.toContain(token);
    expect(out).toBe("url ?k=[REDACTED]&t=[REDACTED] raw [REDACTED] [REDACTED]");
  });
});
