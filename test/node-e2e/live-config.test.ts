// test:live の設定の読み取りと、環境変数がないときの skip を確かめる。
// 実環境には接続しない: 環境変数を持たない実プロセスで live プロジェクトを起動し、全体が skip されることだけを見る。
import { execFile } from "node:child_process";
import { resolve } from "node:path";
import { promisify } from "node:util";
import { describe, expect, it } from "vitest";
import { LiveClient, loadLiveConfig, parseRpcBody, summarize } from "../live/helpers/live";
import { childTmpEnv } from "./tmp";

const run = promisify(execFile);
const REPO = resolve(import.meta.dirname, "../..");

const VALID = {
  BOARD_LIVE_URL: "https://board.example.org/mcp",
  BOARD_LIVE_ACCESS_CLIENT_ID: "abc.access",
  BOARD_LIVE_ACCESS_CLIENT_SECRET: "super-secret-value",
};

describe("loadLiveConfig", () => {
  it("3 つそろっていれば設定を返す", () => {
    const r = loadLiveConfig(VALID);
    expect(r).toEqual({
      ok: true,
      config: {
        url: "https://board.example.org/mcp",
        clientId: "abc.access",
        clientSecret: "super-secret-value",
      },
    });
  });

  it("足りない変数名を挙げて skip の理由にする(値は出さない)", () => {
    const r = loadLiveConfig({ BOARD_LIVE_URL: VALID.BOARD_LIVE_URL });
    expect(r.ok).toBe(false);
    if (r.ok) return;
    expect(r.reason).toContain("BOARD_LIVE_ACCESS_CLIENT_ID");
    expect(r.reason).toContain("BOARD_LIVE_ACCESS_CLIENT_SECRET");
    expect(r.reason).not.toContain("board.example.org");
  });

  it("空白だけの値は未設定として扱う", () => {
    const r = loadLiveConfig({ ...VALID, BOARD_LIVE_URL: "  " });
    expect(r.ok).toBe(false);
  });

  it("URL が https でない・/mcp で終わらないならエラーにする(service token を平文で送らない)", () => {
    for (const url of ["http://board.example.org/mcp", "https://board.example.org/"]) {
      const r = loadLiveConfig({ ...VALID, BOARD_LIVE_URL: url });
      expect(r.ok).toBe(false);
      if (!r.ok) expect(r.reason).toContain("BOARD_LIVE_URL");
    }
  });
});

describe("parseRpcBody / summarize", () => {
  it("JSON と SSE のどちらも読める", () => {
    expect(parseRpcBody('{"result":{"x":1}}')).toEqual({ result: { x: 1 } });
    expect(parseRpcBody('event: message\ndata: {"result":{"x":2}}\n\n')).toEqual({
      result: { x: 2 },
    });
  });

  it("ミリ秒の列から件数・合計・最小・中央値・最大を出す", () => {
    expect(summarize([300, 100, 200])).toEqual({
      n: 3,
      total: 600,
      min: 100,
      median: 200,
      max: 300,
    });
    expect(summarize([])).toEqual({ n: 0, total: 0, min: 0, median: 0, max: 0 });
  });
});

describe("LiveClient", () => {
  it("Access の service token をヘッダで送り、応答時間とサイズを返す。secret はエラーに出さない", async () => {
    const seen: Headers[] = [];
    const fetchImpl = (async (_url: string, init: RequestInit) => {
      seen.push(new Headers(init.headers));
      return new Response(
        JSON.stringify({
          jsonrpc: "2.0",
          id: 1,
          result: { content: [{ type: "text", text: '{"ok":true}' }] },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as unknown as typeof fetch;
    const c = new LiveClient(
      { url: VALID.BOARD_LIVE_URL, clientId: "abc.access", clientSecret: "super-secret-value" },
      fetchImpl,
    );
    const r = await c.call("server_info", {});
    expect(seen[0]?.get("CF-Access-Client-Id")).toBe("abc.access");
    expect(seen[0]?.get("CF-Access-Client-Secret")).toBe("super-secret-value");
    expect(r.isError).toBe(false);
    expect(r.json).toEqual({ ok: true });
    expect(r.bytes).toBeGreaterThan(0);
    expect(r.ms).toBeGreaterThanOrEqual(0);

    const failing = new LiveClient(
      { url: VALID.BOARD_LIVE_URL, clientId: "abc.access", clientSecret: "super-secret-value" },
      (async () =>
        new Response("denied super-secret-value", { status: 403 })) as unknown as typeof fetch,
    );
    const err = await failing.call("server_info", {}).catch((e) => e as Error);
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toContain("403");
    expect((err as Error).message).not.toContain("super-secret-value");
  });
});

describe("mise run test:live の skip", () => {
  it("環境変数がなければ全体を skip し、理由(足りない変数名)を表示して終了コード 0", async () => {
    const { stdout, stderr } = await run(
      resolve(REPO, "node_modules/.bin/vitest"),
      ["run", "--project", "live"],
      { cwd: REPO, env: { PATH: process.env.PATH, HOME: process.env.HOME, ...childTmpEnv() } },
    );
    const out = stdout + stderr;
    expect(out).toContain("BOARD_LIVE_URL");
    expect(out).toMatch(/skipped/i);
  }, 120_000);
});
