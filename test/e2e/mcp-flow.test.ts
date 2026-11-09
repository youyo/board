import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { firstText, readRpc } from "../helpers/mcp";
import { mockOutboundFetch } from "../helpers/mock-fetch";
import { mcpCall } from "../helpers/worker";

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;

let key: TestKey;
let jwt: string;
let outbound: ReturnType<typeof mockOutboundFetch>;

beforeEach(async () => {
  clearJwksCache();
  key = await createTestKey("kid-1");
  jwt = await signAccessJwt(key);
  // BOARD API は登録しない: 未登録の宛先への通信は例外になる(M01 は BOARD を呼ばない)。
  outbound = mockOutboundFetch({ [CERTS]: () => jwksResponse(key) });
});
afterEach(() => {
  vi.restoreAllMocks();
  clearJwksCache();
});

describe("E2E: /mcp (Access の JWT つき)", () => {
  it("initialize → tools/list → tools/call server_info が通る", async () => {
    const init = await mcpCall(jwt, "initialize", {
      protocolVersion: "2025-06-18",
      capabilities: {},
      clientInfo: { name: "e2e", version: "0" },
    });
    expect(init.status).toBe(200);
    expect((await readRpc(init)).result?.serverInfo?.name).toBe("board");

    const list = await readRpc(await mcpCall(jwt, "tools/list", {}, 2));
    const names = list.result?.tools?.map((t) => t.name) ?? [];
    expect(names).toContain("server_info");
    expect(names).toHaveLength(54); // server_info + api 層 40 個(M04)+ find 層 11 個(M05)+ cache 系 2 個(M06)

    const call = await readRpc(
      await mcpCall(jwt, "tools/call", { name: "server_info", arguments: {} }, 3),
    );
    expect(call.result?.isError).not.toBe(true);
    expect(JSON.parse(firstText(call))).toEqual({ name: "board", version: "1.4.0" });
  });

  it("server_info の応答に API キー・トークン・JWT を含めない", async () => {
    const call = await readRpc(
      await mcpCall(jwt, "tools/call", { name: "server_info", arguments: {} }),
    );
    const text = firstText(call);
    for (const secret of ["test-board-api-key", "test-board-api-token", jwt]) {
      expect(text).not.toContain(secret);
    }
  });

  it("未知のツールや不正な引数はエラーエンベロープ(isError)で返る", async () => {
    const bad = await readRpc(
      await mcpCall(jwt, "tools/call", {
        name: "server_info",
        arguments: { max_response_tokens: -1 },
      }),
    );
    expect(bad.result?.isError).toBe(true);
    expect(JSON.parse(firstText(bad)).error.code).toBe("argument_error");
  });

  it("max_response_tokens=1 だと response_too_large のエンベロープ", async () => {
    const res = await readRpc(
      await mcpCall(jwt, "tools/call", {
        name: "server_info",
        arguments: { max_response_tokens: 1 },
      }),
    );
    expect(res.result?.isError).toBe(true);
    expect(JSON.parse(firstText(res)).error.code).toBe("response_too_large");
  });

  it("JWKS は 1 回だけ取得してキャッシュされ、BOARD API には通信しない", async () => {
    await mcpCall(jwt, "tools/list");
    await mcpCall(jwt, "tools/list", {}, 2);
    expect(outbound.callsTo(CERTS)).toHaveLength(1);
    expect(outbound.calls).toHaveLength(1);
  });
});
