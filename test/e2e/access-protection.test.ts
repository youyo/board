import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { createTestKey, jwksResponse, signAccessJwt } from "../helpers/access";
import { MCP_HEADERS } from "../helpers/mcp";
import { mockOutboundFetch } from "../helpers/mock-fetch";
import { callWorker, mcpCall } from "../helpers/worker";

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;

beforeEach(() => clearJwksCache());
afterEach(() => {
  vi.restoreAllMocks();
  clearJwksCache();
});

const PROTECTED: [string, RequestInit?][] = [
  ["/mcp", { method: "POST", body: "{}" }],
  ["/mcp", { method: "GET" }],
  ["/"],
  ["/authorize"],
  ["/token", { method: "POST", body: "" }],
  ["/register", { method: "POST", body: "{}" }],
  ["/.well-known/oauth-authorization-server"],
  ["/anything/else"],
  // 迂回経路の回帰防止: 大文字小文字・末尾スラッシュ・/healthz 配下・GET/POST 以外のメソッド
  ["/MCP", { method: "POST", body: "{}" }],
  ["/Healthz"],
  ["/mcp/", { method: "POST", body: "{}" }],
  ["/healthz/x"],
  ["/mcp", { method: "HEAD" }],
  ["/mcp", { method: "OPTIONS" }],
];

describe("E2E: /healthz 以外はすべて Cf-Access-Jwt-Assertion が必要", () => {
  it.each(PROTECTED)("JWT なしの %s は 401", async (path, init) => {
    const m = mockOutboundFetch({});
    const res = await callWorker(path, { init });
    expect(res.status).toBe(401);
    expect(m.calls).toHaveLength(0);
  });

  it("不正な JWT(期限切れ・aud 違い・署名不正・別チーム・壊れた文字列)は 403 で、JWT もキーも応答に出さない", async () => {
    const key = await createTestKey("kid-1");
    const rogue = await createTestKey("kid-1"); // 同じ kid の別の鍵
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const past = Math.floor(Date.now() / 1000) - 3600;
    const bad = [
      await signAccessJwt(key, { iat: past, exp: past + 300 }),
      await signAccessJwt(key, { aud: "another-app" }),
      await signAccessJwt(rogue),
      await signAccessJwt(key, { teamDomain: "evil.cloudflareaccess.com" }),
      "not-a-jwt",
    ];
    for (const jwt of bad) {
      const res = await mcpCall(jwt, "tools/list");
      expect(res.status, jwt.slice(0, 20)).toBe(403);
      const body = await res.text();
      expect(body).not.toContain(jwt);
      expect(body).not.toContain(key.publicJwk.n ?? "never");
    }
  });

  it("JWKS を取得できないときは fail closed(403)", async () => {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => new Response("boom", { status: 500 }) });
    const res = await mcpCall(await signAccessJwt(key), "tools/list");
    expect(res.status).toBe(403);
  });

  it("不正な JWT の拒否はログに理由の種別だけを残す(JWT を出さない)", async () => {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const spy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const jwt = await signAccessJwt(key, { aud: "another-app" });
    await mcpCall(jwt, "tools/list");
    const logged = spy.mock.calls.map((c) => c.join(" ")).join("\n");
    expect(logged).toContain("invalid");
    expect(logged).not.toContain(jwt);
  });

  it("正しい JWT なら /mcp に到達する(200)", async () => {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const res = await mcpCall(await signAccessJwt(key), "tools/list");
    expect(res.status).toBe(200);
  });

  it("service token の JWT(test:live・Portal の service token 接続の形)でも /mcp に到達する(200)", async () => {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const res = await mcpCall(
      await signAccessJwt(key, { serviceClientId: "abc123.access" }),
      "tools/list",
    );
    expect(res.status).toBe(200);
  });

  it("正しい JWT でも、存在しないパスは 404(認証は通る)", async () => {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const res = await callWorker("/nope", { jwt: await signAccessJwt(key) });
    expect(res.status).toBe(404);
  });
});

describe("E2E: /mcp の Origin 検証と CORS(ブラウザからの呼び出しは想定しない)", () => {
  async function callWithOrigin(origin: string | undefined, method = "POST"): Promise<Response> {
    const key = await createTestKey("kid-1");
    mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
    const headers = new Headers(MCP_HEADERS);
    if (origin !== undefined) headers.set("Origin", origin);
    return callWorker("/mcp", {
      jwt: await signAccessJwt(key),
      init: {
        method,
        headers,
        ...(method === "POST"
          ? { body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/list", params: {} }) }
          : {}),
      },
    });
  }

  const corsHeaderNames = (res: Response) =>
    [...res.headers.keys()].filter((k) => k.toLowerCase().startsWith("access-control-"));

  it("Origin ヘッダーのない正当なリクエスト(MCP クライアント)は通り、CORS ヘッダーは付かない", async () => {
    const res = await callWithOrigin(undefined);
    expect(res.status).toBe(200);
    expect(corsHeaderNames(res)).toEqual([]);
  });

  it.each(["https://evil.example", "http://localhost:3000", "https://board.test"])(
    "Origin が %s のリクエストは 403 で、CORS ヘッダーは付かない",
    async (origin) => {
      const res = await callWithOrigin(origin);
      expect(res.status).toBe(403);
      expect(corsHeaderNames(res)).toEqual([]);
    },
  );

  it("プリフライト(OPTIONS)に CORS の許可を返さない", async () => {
    const res = await callWithOrigin("https://evil.example", "OPTIONS");
    expect(res.headers.get("access-control-allow-origin")).toBeNull();
    expect(corsHeaderNames(res)).toEqual([]);
  });
});
