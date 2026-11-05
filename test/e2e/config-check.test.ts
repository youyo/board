import { afterEach, describe, expect, it, vi } from "vitest";
import { callWorker } from "../helpers/worker";

afterEach(() => vi.restoreAllMocks());

const GATED_PATHS: [string, RequestInit?][] = [
  ["/mcp", { method: "POST", body: "{}" }],
  ["/anything"],
];

describe("E2E: 起動時の設定チェック", () => {
  describe("secret が欠けた Worker", () => {
    it.each(GATED_PATHS)("%s は 500 で、本文に項目名も値も出さない", async (path, init) => {
      vi.spyOn(console, "error").mockImplementation(() => {});
      const res = await callWorker(path, { overrides: { BOARD_API_TOKEN: undefined }, init });
      expect(res.status).toBe(500);
      const body = await res.text();
      expect(body).not.toContain("BOARD_API_TOKEN");
      expect(body).not.toContain("test-board-api-key");
    });

    it("詳細はログに出る(項目名だけ。値は出ない)", async () => {
      const spy = vi.spyOn(console, "error").mockImplementation(() => {});
      await callWorker("/mcp", {
        overrides: { BOARD_API_TOKEN: undefined },
        init: { method: "POST" },
      });
      const logged = spy.mock.calls.map((c) => c.join(" ")).join("\n");
      expect(logged).toContain("BOARD_API_TOKEN");
      expect(logged).not.toContain("test-board-api-key");
    });

    it("JWT なしでも 401 ではなく 500(設定チェックが先)", async () => {
      vi.spyOn(console, "error").mockImplementation(() => {});
      const res = await callWorker("/mcp", {
        overrides: { BOARD_API_KEY: undefined },
        init: { method: "POST", body: "{}" },
      });
      expect(res.status).toBe(500);
    });

    it("/healthz は 503 で、欠けている項目名だけを返す", async () => {
      const res = await callWorker("/healthz", { overrides: { BOARD_API_TOKEN: undefined } });
      expect(res.status).toBe(503);
      expect(await res.json()).toEqual({
        status: "misconfigured",
        missing: ["BOARD_API_TOKEN"],
        invalid: [],
      });
    });
  });

  it("D1 バインディングが無いと /healthz は DB を欠けとして返し、/mcp は 500", async () => {
    const health = await callWorker("/healthz", { overrides: { DB: undefined } });
    expect(health.status).toBe(503);
    expect(((await health.json()) as { missing: string[] }).missing).toEqual(["DB"]);
    vi.spyOn(console, "error").mockImplementation(() => {});
    const mcp = await callWorker("/mcp", {
      overrides: { DB: undefined },
      init: { method: "POST", body: "{}" },
    });
    expect(mcp.status).toBe(500);
  });

  it("ACCESS_AUD がプレースホルダのままなら invalid として /healthz に出る(値は出ない)", async () => {
    const res = await callWorker("/healthz", {
      overrides: { ACCESS_AUD: "<access-application-aud>" },
    });
    expect(res.status).toBe(503);
    const text = JSON.stringify(await res.json());
    expect(text).toContain("ACCESS_AUD");
    expect(text).not.toContain("access-application-aud");
  });

  it("設定がそろっていれば /healthz は 200、/mcp は JWT がなく 401", async () => {
    const health = await callWorker("/healthz");
    expect(health.status).toBe(200);
    expect(await health.json()).toEqual({ status: "ok" });
    const mcp = await callWorker("/mcp", { init: { method: "POST", body: "{}" } });
    expect(mcp.status).toBe(401);
  });
});
