import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type BoardClientOptions,
  createBoardClient,
  createBoardClientFromEnv,
  DEFAULT_MAX_RETRIES,
  DEFAULT_TIMEOUT_MS,
  retryDelayMs,
} from "../../src/board/client";
import { BoardApiError } from "../../src/board/errors";
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
  it("既定値は Go と同じ(最大 5 回再試行・30 秒)", () => {
    expect(DEFAULT_MAX_RETRIES).toBe(5);
    expect(DEFAULT_TIMEOUT_MS).toBe(30_000);
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
    expect(m.calls).toHaveLength(6);
    expect(waits).toHaveLength(5);
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
    await client.get("/v1/clients");
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
    // 500, 1000, 2000, 4000, 8000 の手前(floor(exp * 0.999999))
    expect(waits).toEqual([499, 999, 1999, 3999, 7999]);
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
});
