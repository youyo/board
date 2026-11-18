/**
 * feedback ツール(ADR 0007。共通仕様は logbutler の ADR 0019)。Worker 全体への HTTP(/mcp)で、tools/list と tools/call の通しと、
 * 応答に環境の値が出ないことを固定する。
 *
 * 応答はそのまま公開 issue に貼られうる。env(BOARD_BASE_URL・BOARD_UI_BASE_URL・Access の設定・API キーとトークン)と
 * 利用者(JWT の email)が異なる 2 つの構成で、kind ごとに応答が完全に一致することを確かめる(C5)。
 * 外向きの通信は Access の JWKS だけ(BOARD API は登録しない。呼べば例外になる)。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { buildFeedbackGuide, FEEDBACK_KINDS } from "../../src/feedback/guide";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { firstText, MCP_HEADERS, readRpc } from "../helpers/mcp";
import { mockOutboundFetch } from "../helpers/mock-fetch";
import { callWorker, mcpCall } from "../helpers/worker";

const NAME = "board_feedback";

/** 構成 A: テストの既定(vitest.config.ts の束縛)。 */
const A = {
  team: "board-test.cloudflareaccess.com",
  aud: "test-access-aud",
  email: "alice@example.com",
};

/** 構成 B: Access・BOARD の接続先・API キー・利用者を、構成 A と変えた番兵の値。 */
const B = {
  team: "sentinel-team-k3v.cloudflareaccess.com",
  aud: "sentinel-aud-77w",
  email: "sentinel-user-z8m@sentinel-corp.example",
  boardBase: "https://api.sentinel-board-q7x.example.net",
  uiBase: "https://sentinel-ui-m2z.example.net",
  apiKey: "test-board-api-key-sentinel-b",
  apiToken: "sentinel-api-token-81p",
};

const overridesB = {
  ACCESS_TEAM_DOMAIN: B.team,
  ACCESS_AUD: B.aud,
  BOARD_BASE_URL: B.boardBase,
  BOARD_UI_BASE_URL: B.uiBase,
  BOARD_API_KEY: B.apiKey,
  BOARD_API_TOKEN: B.apiToken,
};

let key: TestKey;
let jwtA: string;
let jwtB: string;
let outbound: ReturnType<typeof mockOutboundFetch>;

beforeEach(async () => {
  clearJwksCache();
  key = await createTestKey("kid-1");
  jwtA = await signAccessJwt(key, { teamDomain: A.team, aud: A.aud, email: A.email });
  jwtB = await signAccessJwt(key, { teamDomain: B.team, aud: B.aud, email: B.email });
  // 外向きの通信は JWKS だけ。BOARD API は登録しない(通信すれば例外になり、呼び出しが失敗する)。
  outbound = mockOutboundFetch({
    [`GET https://${A.team}/cdn-cgi/access/certs`]: () => jwksResponse(key),
    [`GET https://${B.team}/cdn-cgi/access/certs`]: () => jwksResponse(key),
  });
});
afterEach(() => {
  vi.restoreAllMocks();
  clearJwksCache();
});

/** 構成 B の env で /mcp に JSON-RPC を 1 件送る。 */
function mcpCallB(method: string, params: Record<string, unknown> = {}, id = 1) {
  return callWorker("/mcp", {
    jwt: jwtB,
    overrides: overridesB,
    init: {
      method: "POST",
      headers: MCP_HEADERS,
      body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
    },
  });
}

const callFeedbackA = async (args: Record<string, unknown>) =>
  readRpc(await mcpCall(jwtA, "tools/call", { name: NAME, arguments: args }));
const callFeedbackB = async (args: Record<string, unknown>) =>
  readRpc(await mcpCallB("tools/call", { name: NAME, arguments: args }));

describe("E2E: board_feedback の公開", () => {
  it("tools/list に出て、読み取り専用・冪等・openWorldHint=false で、入力スキーマは kind だけ", async () => {
    const res = await readRpc(await mcpCall(jwtA, "tools/list"));
    const tool = res.result?.tools?.find((t) => t.name === NAME) as unknown as
      | {
          title?: string;
          annotations: Record<string, unknown>;
          inputSchema: { properties: Record<string, unknown> };
        }
      | undefined;
    expect(tool).toBeDefined();
    expect(tool?.annotations).toEqual({
      title: "フィードバック（GitHub issue の起票案内）",
      readOnlyHint: true,
      idempotentHint: true,
      openWorldHint: false,
    });
    expect(Object.keys(tool?.inputSchema.properties ?? {})).toEqual(["kind"]);
  });

  it("tools/call が通り、省略時は bug。JSON の text content で返す", async () => {
    const rpc = await callFeedbackA({});
    expect(rpc.result?.isError).toBeUndefined();
    expect(JSON.parse(firstText(rpc))).toEqual(buildFeedbackGuide("bug"));
    expect((rpc.result as Record<string, unknown>).structuredContent).toBeUndefined();
    // 外向きの通信は JWKS だけ
    expect(outbound.calls.map((c) => c.url)).toEqual([`https://${A.team}/cdn-cgi/access/certs`]);
  });

  it("不正な kind は argument_error のエンベロープで、値をそのまま返さない", async () => {
    const rpc = await callFeedbackA({ kind: "sentinel-kind-https://api.the-board.jp/v1/x" });
    expect(rpc.result?.isError).toBe(true);
    const text = firstText(rpc);
    expect(JSON.parse(text)).toMatchObject({
      schema_version: "1",
      error: { code: "argument_error", retryable: false },
    });
    expect(text).not.toContain("sentinel");
  });

  it("宣言していない引数(tool_name・下書きの title / body)は無視され、応答に出ない", async () => {
    const plain = firstText(await callFeedbackA({ kind: "bug" }));
    const extra = firstText(
      await callFeedbackA({
        kind: "bug",
        tool_name: "sentinel_tool_q1",
        title: "sentinel-title-q2",
        body: "sentinel-body-q3",
      }),
    );
    expect(extra).toBe(plain);
    expect(extra).not.toContain("sentinel");
  });
});

describe("E2E: board_feedback の応答は環境に依存しない(C5)", () => {
  it.each([undefined, ...FEEDBACK_KINDS])(
    "kind=%s: env と利用者が異なる 2 つの構成で応答が完全に一致し、どちらの環境の値も出ない",
    async (kind) => {
      const args = kind === undefined ? {} : { kind };
      const textA = firstText(await callFeedbackA(args));
      const textB = firstText(await callFeedbackB(args));
      expect(textA).not.toBe("");
      expect(textB).toBe(textA);
      expect(textA).toBe(JSON.stringify(buildFeedbackGuide(kind ?? "bug")));

      const sentinels = [
        // 構成 A(テストの既定)
        A.team,
        A.aud,
        A.email,
        "api.board.test",
        "test-board-api-key",
        "test-board-api-token",
        jwtA,
        // 構成 B
        "sentinel",
        B.team,
        B.aud,
        B.email,
        B.boardBase,
        "api.sentinel-board-q7x.example.net",
        B.uiBase,
        "sentinel-ui-m2z.example.net",
        B.apiKey,
        B.apiToken,
        jwtB,
      ];
      for (const s of sentinels) {
        expect(textA, `kind=${kind} に ${s}`).not.toContain(s);
      }
    },
  );

  it("構成 B の呼び出しが通る(構成 B の JWKS だけを取りに行き、BOARD API には通信しない)", async () => {
    await callFeedbackB({});
    const urls = outbound.calls.map((c) => c.url);
    expect(urls).toEqual([`https://${B.team}/cdn-cgi/access/certs`]);
  });
});
