/**
 * BOARD API クライアント(Go 版 `internal/boardapi/client.go` `auth.go` `retry.go` の移植)。
 *
 * - 認証: `x-api-key: <BOARD_API_KEY>` と `Authorization: Bearer <BOARD_API_TOKEN>`。GET のみ(読み取り専用)。
 * - 直列化: 1 クライアントの中では同時に 1 リクエスト(再試行の待機を含む)。BOARD は 3 回/秒の制限がある。
 * - 再試行: 429・5xx・通信失敗を最大 5 回。`Retry-After`(整数秒)を優先し 60 秒で頭打ち。なければ指数バックオフ + full jitter。
 * - タイムアウト: 1 試行 30 秒。応答ヘッダーだけでなく本文の読み取りまで同じ期限で守る。
 * - 秘密情報: キー・トークンはエラーメッセージにもログにも出さない(通信の例外は原因を捨てて固定文にする。クライアントはログを出さない)。
 * - リダイレクトは追わない(`x-api-key` が別のオリジンに送られないように)。
 */

import type { Env } from "../env";
import { BoardError } from "../errors";
import { BoardApiError, type BoardApiErrorDetail } from "./errors";

export { BoardApiError };

/** 1 試行のタイムアウト(Go の defaultTimeout)。 */
export const DEFAULT_TIMEOUT_MS = 30_000;
/** 再試行の上限回数(Go の retryMax。最大 6 試行)。 */
export const DEFAULT_MAX_RETRIES = 5;
/** 応答本文の上限(Go の `io.LimitReader` 32MiB)。 */
export const DEFAULT_MAX_RESPONSE_BYTES = 32 * 1024 * 1024;

const RETRY_BASE_DELAY_MS = 500;
const RETRY_MAX_DELAY_MS = 30_000;
const RETRY_MAX_RETRY_AFTER_SECONDS = 60;
/** エラー本文の message をメッセージに載せるときの上限文字数。 */
const MAX_ERROR_MESSAGE_CHARS = 500;

export interface BoardClientOptions {
  /** `https://api.the-board.jp` のようなオリジン(末尾のスラッシュは除く)。 */
  baseUrl: string;
  apiKey: string;
  apiToken: string;
  timeoutMs?: number;
  maxRetries?: number;
  maxResponseBytes?: number;
  /** 待機の差し替え口(テスト用)。既定は setTimeout。 */
  sleep?: (ms: number) => Promise<void>;
  /** jitter の乱数(テスト用)。[0, 1) を返す。既定は Math.random。 */
  random?: () => number;
}

export interface BoardResponse {
  /** JSON.parse した本文(検証しない。BOARD の生データ)。 */
  body: unknown;
  headers: Headers;
}

export interface BoardClient {
  /** `path` は `/v1/...`。`query` は `?` を除いたクエリ文字列(`QueryBuilder.encode()` の結果)。 */
  get(path: string, query?: string): Promise<BoardResponse>;
  /** 文字列に紛れたキー・トークンを伏せる。 */
  redact(text: string): string;
}

const defaultSleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * 再試行までの待機(ミリ秒)。Go の CalcBackoff。
 * `Retry-After`(1 秒以上)があれば 60 秒を上限にそれを使う。なければ 500ms × 2^attempt(上限 30 秒)の full jitter。
 */
export function retryDelayMs(
  attempt: number,
  retryAfterSeconds: number | undefined,
  random: () => number = Math.random,
): number {
  if (retryAfterSeconds !== undefined && retryAfterSeconds > 0) {
    return Math.min(retryAfterSeconds, RETRY_MAX_RETRY_AFTER_SECONDS) * 1000;
  }
  const exp = Math.min(RETRY_BASE_DELAY_MS * 2 ** attempt, RETRY_MAX_DELAY_MS);
  return Math.floor(random() * exp);
}

/** Retry-After(整数秒)を読む。日付形式・小数・負数・空は undefined(Go は strconv.Atoi)。 */
function parseRetryAfter(header: string | null): number | undefined {
  const v = header?.trim();
  return v && /^\d+$/.test(v) ? Number(v) : undefined;
}

const abortError = () => new DOMException("aborted", "AbortError");

/** 本文を読む。signal が中断されたら即座に失敗しストリームを cancel する。maxBytes を超えたら BoardError。 */
async function readBody(res: Response, signal: AbortSignal, maxBytes: number): Promise<string> {
  if (!res.body) return "";
  const reader = res.body.getReader();
  const aborted = new Promise<never>((_, reject) => {
    if (signal.aborted) reject(abortError());
    signal.addEventListener("abort", () => reject(abortError()), { once: true });
  });
  aborted.catch(() => undefined);
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    for (;;) {
      const { done, value } = await Promise.race([reader.read(), aborted]);
      if (done) break;
      total += value.byteLength;
      if (total > maxBytes) {
        throw new BoardError("api_error", "BOARD API response is too large.", false);
      }
      chunks.push(value);
    }
  } catch (e) {
    await reader.cancel().catch(() => undefined);
    throw e;
  }
  const out = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    out.set(c, offset);
    offset += c.byteLength;
  }
  return new TextDecoder().decode(out);
}

export function createBoardClient(opts: BoardClientOptions): BoardClient {
  const baseUrl = opts.baseUrl.replace(/\/+$/, "");
  const timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const maxRetries = opts.maxRetries ?? DEFAULT_MAX_RETRIES;
  const maxResponseBytes = opts.maxResponseBytes ?? DEFAULT_MAX_RESPONSE_BYTES;
  const sleep = opts.sleep ?? defaultSleep;
  const random = opts.random ?? Math.random;
  const headers = {
    "x-api-key": opts.apiKey,
    Authorization: `Bearer ${opts.apiToken}`,
    Accept: "application/json",
  };

  // 直列化: 前のリクエスト(再試行の待機を含む)が終わるまで次を始めない。失敗しても後続は続ける。
  let tail: Promise<unknown> = Promise.resolve();
  function serialize<T>(run: () => Promise<T>): Promise<T> {
    const p = tail.then(run);
    tail = p.catch(() => undefined);
    return p;
  }

  /** BOARD の本文や例外に紛れたキー・トークンを伏せる。 */
  const redact = (text: string): string => {
    let out = text;
    for (const secret of [opts.apiKey, opts.apiToken]) {
      if (secret) out = out.split(secret).join("[REDACTED]");
    }
    return out;
  };

  /** エラー応答の本文から message(なければ error)を取る。読めなければ undefined。 */
  async function errorDetail(res: Response, signal: AbortSignal): Promise<BoardApiErrorDetail> {
    const detail: BoardApiErrorDetail = {
      retryAfterSeconds: parseRetryAfter(res.headers.get("Retry-After")),
    };
    try {
      const parsed = JSON.parse(await readBody(res, signal, maxResponseBytes)) as {
        message?: unknown;
        error?: unknown;
      } | null;
      const m =
        typeof parsed?.message === "string" && parsed.message
          ? parsed.message
          : typeof parsed?.error === "string"
            ? parsed.error
            : "";
      if (m) detail.boardMessage = redact(m).slice(0, MAX_ERROR_MESSAGE_CHARS);
    } catch {
      // JSON でない本文・読み取りの失敗(期限切れを含む)は無視して status だけで変換する。
    }
    return detail;
  }

  /** 1 試行。本文を読み終えるまで同じ期限(timeoutMs)で守る。 */
  async function attempt(url: string): Promise<BoardResponse> {
    const controller = new AbortController();
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, timeoutMs);
    // 例外の文面にキー・トークンやヘッダーが混ざらないよう、通信の原因は捨てて固定文にする。
    const transportError = () =>
      new BoardApiError(0, {
        reason: timedOut
          ? `BOARD API request timed out after ${timeoutMs}ms.`
          : "BOARD API is unreachable.",
      });
    try {
      let res: Response;
      try {
        res = await fetch(url, {
          method: "GET",
          headers,
          redirect: "manual",
          signal: controller.signal,
        });
      } catch {
        throw transportError();
      }
      if (!res.ok) {
        throw new BoardApiError(res.status, await errorDetail(res, controller.signal));
      }
      let text: string;
      try {
        text = await readBody(res, controller.signal, maxResponseBytes);
      } catch (e) {
        if (e instanceof BoardError) throw e;
        throw transportError();
      }
      try {
        return { body: JSON.parse(text), headers: res.headers };
      } catch {
        throw new BoardError("api_error", "BOARD API returned a non-JSON response.", false);
      }
    } finally {
      clearTimeout(timer);
    }
  }

  async function execute(url: string): Promise<BoardResponse> {
    for (let n = 0; ; n++) {
      try {
        return await attempt(url);
      } catch (e) {
        if (!(e instanceof BoardApiError) || !e.retryable || n >= maxRetries) throw e;
        await sleep(retryDelayMs(n, e.retryAfterSeconds, random));
      }
    }
  }

  return {
    get(path, query) {
      const url = `${baseUrl}${path}${query ? `?${query}` : ""}`;
      return serialize(() => execute(url));
    },
    redact,
  };
}

/** Worker の環境変数・secret からクライアントを作る。 */
export function createBoardClientFromEnv(
  env: Pick<Env, "BOARD_BASE_URL" | "BOARD_API_KEY" | "BOARD_API_TOKEN">,
  overrides: Partial<BoardClientOptions> = {},
): BoardClient {
  return createBoardClient({
    baseUrl: env.BOARD_BASE_URL,
    apiKey: env.BOARD_API_KEY,
    apiToken: env.BOARD_API_TOKEN,
    ...overrides,
  });
}
