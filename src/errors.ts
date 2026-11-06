/**
 * ツールのエラー。エンベロープ `{"schema_version":"1","error":{"code","message","retryable"}}` に変換する。
 */

export const ERROR_CODES = [
  "argument_error",
  "authentication_error",
  "permission_error",
  "not_found",
  "api_error",
  "rate_limited",
  "refresh_in_progress",
  "response_too_large",
  "generic_error",
] as const;
export type ErrorCode = (typeof ERROR_CODES)[number];

/** ツールの応答に載せてよいエラー。message は利用者(AI)にそのまま見える。API キー・トークンを含めないこと。 */
export class BoardError extends Error {
  constructor(
    readonly code: ErrorCode,
    message: string,
    readonly retryable = false,
  ) {
    super(message);
    this.name = new.target.name;
  }
}

/** 入力が不正(スキーマでは表せない検証)。 */
export class ArgumentError extends BoardError {
  constructor(message: string) {
    super("argument_error", message, false);
  }
}

/**
 * 応答が上限(ツール引数 max_response_tokens)を超えた。切り詰めずにエラーにする。
 * 絞り込んでも同じ条件では直らないので retryable は false。
 */
export class ResponseTooLargeError extends BoardError {
  constructor(estimatedTokens: number, limit: number) {
    super(
      "response_too_large",
      `Response is too large: estimated size ${estimatedTokens} tokens exceeds the limit of ${limit} tokens. ` +
        "Narrow the request and retry (e.g. use filters, or set the limit argument to cap the number of items), " +
        "or raise max_response_tokens (0 disables the limit).",
      false,
    );
  }
}

/** 同じリソースの refresh が別の呼び出しで進行中(M03)。待たずに即返し、`retry_after_seconds` 後の再試行を促す。 */
export const REFRESH_RETRY_AFTER_SECONDS = 30;

export class RefreshInProgressError extends BoardError {
  readonly retryAfterSeconds = REFRESH_RETRY_AFTER_SECONDS;
  readonly resource: string;
  readonly holder: string;
  readonly elapsedSeconds: number;

  constructor(resource: string, info: { holder?: string; elapsedSeconds?: number } = {}) {
    const holder = info.holder ?? "";
    const elapsedSeconds = info.elapsedSeconds ?? 0;
    super(
      "refresh_in_progress",
      // 保持者の識別子(UUID)は利用者に見せない。フィールド(holder)には残す。
      `Refresh in progress for ${resource} (elapsed=${elapsedSeconds}s). ` +
        `Retry after ${REFRESH_RETRY_AFTER_SECONDS} seconds.`,
      true,
    );
    this.resource = resource;
    this.holder = holder;
    this.elapsedSeconds = elapsedSeconds;
  }
}

export interface ErrorEnvelope {
  schema_version: "1";
  error: { code: ErrorCode; message: string; retryable: boolean; retry_after_seconds?: number };
}

export function errorEnvelope(code: ErrorCode, message: string, retryable: boolean): ErrorEnvelope {
  return { schema_version: "1", error: { code, message, retryable } };
}

export function toErrorEnvelope(e: unknown): ErrorEnvelope {
  if (e instanceof RefreshInProgressError) {
    const env = errorEnvelope(e.code, e.message, e.retryable);
    env.error.retry_after_seconds = e.retryAfterSeconds;
    return env;
  }
  if (e instanceof BoardError) return errorEnvelope(e.code, e.message, e.retryable);
  // 想定外の例外は内部情報を漏らさない。
  return errorEnvelope("generic_error", "Unexpected error.", false);
}
