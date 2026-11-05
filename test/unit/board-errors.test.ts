import { describe, expect, it } from "vitest";
import { BoardApiError, errorCodeForStatus, isRetryableStatus } from "../../src/board/errors";
import { toErrorEnvelope } from "../../src/errors";

describe("BOARD のステータス → エラー code (Go の ClassifyStatusCode と logbutler の列挙)", () => {
  it.each([
    [400, "argument_error"],
    [401, "authentication_error"],
    [403, "permission_error"],
    [404, "not_found"],
    [422, "argument_error"],
    [429, "rate_limited"],
    [500, "api_error"],
    [503, "api_error"],
    [409, "api_error"],
    [302, "api_error"],
    [0, "api_error"],
  ] as const)("HTTP %i → %s", (status, code) => {
    expect(errorCodeForStatus(status)).toBe(code);
  });

  it("再試行できるのは通信失敗(0)・429・5xx だけ(Go の isRetryable)", () => {
    for (const s of [0, 429, 500, 502, 503, 599]) expect(isRetryableStatus(s)).toBe(true);
    for (const s of [400, 401, 403, 404, 422, 409]) expect(isRetryableStatus(s)).toBe(false);
  });
});

describe("BoardApiError", () => {
  it("BOARD の message をメッセージに載せる。エンベロープに変換できる", () => {
    const e = new BoardApiError(404, { boardMessage: "Couldn't find Client" });
    expect(e.status).toBe(404);
    expect(toErrorEnvelope(e)).toEqual({
      schema_version: "1",
      error: {
        code: "not_found",
        message: "BOARD API error (HTTP 404): Couldn't find Client",
        retryable: false,
      },
    });
  });

  it("message がなければステータスだけ", () => {
    expect(new BoardApiError(500).message).toBe("BOARD API error (HTTP 500).");
    expect(new BoardApiError(500).retryable).toBe(true);
  });

  it("status 0 は reason(固定文)を使う", () => {
    const e = new BoardApiError(0, { reason: "BOARD API request timed out after 30000ms." });
    expect(e.message).toBe("BOARD API request timed out after 30000ms.");
    expect(e.code).toBe("api_error");
    expect(e.retryable).toBe(true);
    expect(new BoardApiError(0).message).toBe("BOARD API is unreachable.");
  });

  it("Retry-After を秒で持つ(メッセージには出さない)", () => {
    const e = new BoardApiError(429, { retryAfterSeconds: 7 });
    expect(e.retryAfterSeconds).toBe(7);
    expect(e.code).toBe("rate_limited");
    expect(e.message).toBe("BOARD API error (HTTP 429).");
  });
});
