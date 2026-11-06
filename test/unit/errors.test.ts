import { describe, expect, it } from "vitest";
import {
  ArgumentError,
  BoardError,
  ERROR_CODES,
  errorEnvelope,
  ResponseTooLargeError,
  toErrorEnvelope,
} from "../../src/errors";

describe("エラーエンベロープ", () => {
  it("errorEnvelope は {schema_version, error:{code,message,retryable}} の形", () => {
    expect(errorEnvelope("not_found", "no such thing", false)).toEqual({
      schema_version: "1",
      error: { code: "not_found", message: "no such thing", retryable: false },
    });
  });

  it("エラーコードの列挙", () => {
    expect([...ERROR_CODES]).toEqual([
      "argument_error",
      "authentication_error",
      "permission_error",
      "not_found",
      "api_error",
      "rate_limited",
      "refresh_in_progress",
      "response_too_large",
      "generic_error",
    ]);
  });

  it("BoardError はコード・message・retryable をそのまま載せる", () => {
    expect(toErrorEnvelope(new BoardError("rate_limited", "slow down", true))).toEqual({
      schema_version: "1",
      error: { code: "rate_limited", message: "slow down", retryable: true },
    });
  });

  it("ArgumentError は argument_error・retryable=false", () => {
    expect(toErrorEnvelope(new ArgumentError("bad"))).toEqual({
      schema_version: "1",
      error: { code: "argument_error", message: "bad", retryable: false },
    });
  });

  it("ResponseTooLargeError は response_too_large・retryable=false で、推定サイズと上限を message に含む", () => {
    const env = toErrorEnvelope(new ResponseTooLargeError(1234, 100));
    expect(env.error.code).toBe("response_too_large");
    expect(env.error.retryable).toBe(false);
    expect(env.error.message).toContain("1234");
    expect(env.error.message).toContain("100");
    expect(env.error.message).toContain("max_response_tokens");
    // 絞り込みに加えて limit を案内する(M04 指摘)。
    expect(env.error.message).toContain("limit argument");
    expect(env.error.message).toContain("filters");
  });

  it("想定外の例外は内部情報(message)を漏らさず generic_error にする", () => {
    const env = toErrorEnvelope(new Error("secret-api-key-123 leaked"));
    expect(env.error.code).toBe("generic_error");
    expect(env.error.retryable).toBe(false);
    expect(JSON.stringify(env)).not.toContain("secret-api-key-123");
    expect(toErrorEnvelope("string thrown").error.code).toBe("generic_error");
  });
});

describe("refresh_in_progress(M03 で追加した仕様)", () => {
  it("エンベロープに retry_after_seconds を載せる(他のエラーには載せない)", async () => {
    const { RefreshInProgressError } = await import("../../src/errors");
    const e = new RefreshInProgressError("clients", { holder: "h", elapsedSeconds: 3 });
    expect(toErrorEnvelope(e)).toEqual({
      schema_version: "1",
      error: {
        code: "refresh_in_progress",
        message: e.message,
        retryable: true,
        retry_after_seconds: 30,
      },
    });
    expect(toErrorEnvelope(new BoardError("not_found", "x", false)).error).not.toHaveProperty(
      "retry_after_seconds",
    );
  });

  it("文言にロック保持者(UUID)を出さない。保持者は例外のフィールドに残る", async () => {
    const { RefreshInProgressError } = await import("../../src/errors");
    const holder = "3f2b8c1e-aaaa-4bbb-8ccc-0123456789ab";
    const e = new RefreshInProgressError("clients", { holder, elapsedSeconds: 3 });
    expect(e.message).not.toContain(holder);
    expect(e.message).not.toContain("holder");
    expect(e.message).toContain("clients");
    expect(e.holder).toBe(holder);
    expect(JSON.stringify(toErrorEnvelope(e))).not.toContain(holder);
  });
});
