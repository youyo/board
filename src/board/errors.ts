import { BoardError, type ErrorCode } from "../errors";

/** HTTP ステータス → エラー code。Go の ClassifyStatusCode(400 と 422 は VALIDATION)に logbutler の列挙を当てる。 */
export function errorCodeForStatus(status: number): ErrorCode {
  switch (status) {
    case 400:
    case 422:
      return "argument_error";
    case 401:
      return "authentication_error";
    case 403:
      return "permission_error";
    case 404:
      return "not_found";
    case 429:
      return "rate_limited";
    default:
      return "api_error";
  }
}

/** 再試行で直る可能性がある: 通信失敗(status 0)・429・5xx (Go の isRetryable)。 */
export function isRetryableStatus(status: number): boolean {
  return status === 0 || status === 429 || status >= 500;
}

export interface BoardApiErrorDetail {
  /** BOARD のエラー本文の `message`(なければ `error`)。キー・トークンは伏せ済みであること。 */
  boardMessage?: string;
  /** 通信失敗・タイムアウトなど、HTTP 応答がないときの固定文。 */
  reason?: string;
  /** `Retry-After`(整数秒、1 以上)。 */
  retryAfterSeconds?: number;
}

/**
 * BOARD API のエラー。status 0 は「HTTP 応答を得られなかった」(通信失敗・タイムアウト)。
 * message にはキー・トークン・リクエストヘッダーを含めない。
 */
export class BoardApiError extends BoardError {
  readonly retryAfterSeconds?: number;

  constructor(
    readonly status: number,
    detail: BoardApiErrorDetail = {},
  ) {
    super(errorCodeForStatus(status), describe(status, detail), isRetryableStatus(status));
    this.retryAfterSeconds = detail.retryAfterSeconds;
  }
}

function describe(status: number, d: BoardApiErrorDetail): string {
  if (status === 0) return d.reason ?? "BOARD API is unreachable.";
  const head = `BOARD API error (HTTP ${status})`;
  return d.boardMessage ? `${head}: ${d.boardMessage}` : `${head}.`;
}

/**
 * 呼び出しの締め切りまでに終わらなかった(M02 追補)。元が 429 なら rate_limited、それ以外は api_error。
 * refresh が『ここまでの進捗を保存して partial で返す』判断のために、文面ではなくこの型で見分ける。
 */
export class DeadlineExceededError extends BoardError {
  constructor(rateLimited: boolean) {
    super(
      rateLimited ? "rate_limited" : "api_error",
      "BOARD API did not respond before the deadline.",
      true,
    );
  }
}

export function isDeadlineError(e: unknown): e is DeadlineExceededError {
  return e instanceof DeadlineExceededError;
}
