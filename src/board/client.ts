/**
 * BOARD API クライアント(Go 版 `internal/boardapi/client.go` `auth.go` `retry.go` の移植)。
 *
 * - 認証: `x-api-key: <BOARD_API_KEY>` と `Authorization: Bearer <BOARD_API_TOKEN>`。GET のみ(読み取り専用)。
 * - 直列化: 1 クライアントの中では同時に 1 リクエスト(再試行の待機を含む)。BOARD は 3 回/秒の制限がある。
 * - 再試行: 429・5xx・通信失敗を最大 3 回。`Retry-After`(整数秒)を優先し 60 秒で頭打ち。なければ指数バックオフ + full jitter。
 * - タイムアウト: 1 試行は min(30 秒, 残り時間)。応答ヘッダーだけでなく本文の読み取りまで同じ期限で守る。
 * - 締め切りとキャンセル: 呼び出し単位の `CallContext`(deadline / signal)。キュー待ち・fetch・待機のすべてに効く。
 *   待機が残り時間を超えるなら待たずに失敗させる。
 * - 秘密情報: キー・トークンはエラーメッセージにもログにも出さない(通信の例外は原因を捨てて固定文にする。クライアントはログを出さない)。
 * - リダイレクトは追わない(`x-api-key` が別のオリジンに送られないように)。
 */

import type { Env } from "../env";
import { BoardError } from "../errors";
import { type CallContext, DEFAULT_CALL_BUDGET_MS, resolveContext } from "./context";
import { BoardApiError, type BoardApiErrorDetail, DeadlineExceededError } from "./errors";

export type { CallContext };
export { BoardApiError, DEFAULT_CALL_BUDGET_MS };

/** 1 試行のタイムアウト(Go の defaultTimeout)。 */
export const DEFAULT_TIMEOUT_MS = 30_000;
/** 再試行の上限回数(最大 4 試行)。Go の retryMax は 5 だが CLI 前提の値なので 3 にした。 */
export const DEFAULT_MAX_RETRIES = 3;
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
  /** 待機の差し替え口(テスト用)。既定は setTimeout。signal が abort されたら待機をやめる。 */
  sleep?: (ms: number, signal?: AbortSignal) => Promise<void>;
  /** 現在時刻(epoch ms。テスト用)。既定は Date.now。 */
  now?: () => number;
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
  get(path: string, query?: string, ctx?: CallContext): Promise<BoardResponse>;
  /** 文字列に紛れたキー・トークンを伏せる。 */
  redact(text: string): string;
}

const abortError = () => new DOMException("aborted", "AbortError");

const defaultSleep = (ms: number, signal?: AbortSignal) =>
  new Promise<void>((resolve, reject) => {
    if (signal?.aborted) return reject(abortError());
    const onAbort = () => {
      clearTimeout(timer);
      reject(abortError());
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });

/** 呼び出し元が中断した。 */
const callAborted = () => new BoardError("api_error", "BOARD API call was aborted.", true);

/** 締め切りまでに終わらなかった。元が 429 なら rate_limited、それ以外は api_error。 */
const deadlineExceeded = (rateLimited: boolean) => new DeadlineExceededError(rateLimited);

/** promise を signal の abort と競わせる。abort されたら即座に reject する(元の promise は放置)。 */
function raceAbort<T>(promise: Promise<T>, signal: AbortSignal | undefined): Promise<T> {
  if (!signal) return promise;
  if (signal.aborted) {
    promise.catch(() => undefined);
    return Promise.reject(abortError());
  }
  return new Promise<T>((resolve, reject) => {
    const onAbort = () => reject(abortError());
    signal.addEventListener("abort", onAbort, { once: true });
    promise.then(
      (v) => {
        signal.removeEventListener("abort", onAbort);
        resolve(v);
      },
      (e) => {
        signal.removeEventListener("abort", onAbort);
        reject(e);
      },
    );
  });
}

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
  const now = opts.now ?? Date.now;
  const random = opts.random ?? Math.random;
  const headers = {
    "x-api-key": opts.apiKey,
    Authorization: `Bearer ${opts.apiToken}`,
    Accept: "application/json",
  };

  // 直列化: 前のリクエスト(再試行の待機を含む)が終わるまで次を始めない。失敗しても後続は続ける。
  // 順番待ちも締め切りと abort の対象。待ちをやめた呼び出しは自分の枠をすぐ手放し、後続を塞がない。
  let tail: Promise<void> = Promise.resolve();
  async function serialize<T>(
    run: () => Promise<T>,
    deadline: number,
    signal: AbortSignal | undefined,
  ): Promise<T> {
    const prev = tail;
    let release!: () => void;
    const mine = new Promise<void>((resolve) => {
      release = resolve;
    });
    tail = prev.then(() => mine);
    try {
      let timer: ReturnType<typeof setTimeout> | undefined;
      const expired = new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(deadlineExceeded(false)), Math.max(0, deadline - now()));
      });
      try {
        await Promise.race([raceAbort(prev, signal), expired]);
      } catch (e) {
        throw e instanceof BoardError ? e : callAborted();
      } finally {
        clearTimeout(timer);
      }
      return await run();
    } finally {
      release();
    }
  }

  /** BOARD の本文や例外に紛れたキー・トークンを伏せる。 */
  const redact = (text: string): string => {
    let out = text;
    for (const secret of [opts.apiKey, opts.apiToken]) {
      if (!secret) continue;
      // 生の値と、URL に載ったときの encodeURIComponent した形の両方を伏せる。
      for (const form of new Set([secret, encodeURIComponent(secret)])) {
        out = out.split(form).join("[REDACTED]");
      }
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

  /** 1 試行。本文を読み終えるまで同じ期限(attemptMs)で守る。呼び出し元の signal でも中断する。 */
  async function attempt(
    url: string,
    attemptMs: number,
    signal: AbortSignal | undefined,
  ): Promise<BoardResponse> {
    const controller = new AbortController();
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, attemptMs);
    const onCallerAbort = () => controller.abort();
    if (signal?.aborted) controller.abort();
    else signal?.addEventListener("abort", onCallerAbort, { once: true });
    // 例外の文面にキー・トークンやヘッダーが混ざらないよう、通信の原因は捨てて固定文にする。
    const transportError = () =>
      signal?.aborted
        ? callAborted()
        : new BoardApiError(0, {
            reason: timedOut
              ? `BOARD API request timed out after ${attemptMs}ms.`
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
      signal?.removeEventListener("abort", onCallerAbort);
    }
  }

  async function execute(
    url: string,
    deadline: number,
    signal: AbortSignal | undefined,
  ): Promise<BoardResponse> {
    let last: BoardApiError | undefined;
    for (let n = 0; ; n++) {
      if (signal?.aborted) throw callAborted();
      const remaining = deadline - now();
      if (remaining <= 0) throw deadlineExceeded(last?.status === 429);
      try {
        return await attempt(url, Math.min(timeoutMs, remaining), signal);
      } catch (e) {
        if (signal?.aborted) throw callAborted();
        if (!(e instanceof BoardApiError) || !e.retryable || n >= maxRetries) throw e;
        last = e;
        const delay = retryDelayMs(n, e.retryAfterSeconds, random);
        // 待っても締め切りに間に合わないなら、待たずに失敗させる。
        if (delay >= deadline - now()) throw deadlineExceeded(e.status === 429);
        try {
          await raceAbort(sleep(delay, signal), signal);
        } catch {
          throw callAborted();
        }
      }
    }
  }

  return {
    get(path, query, ctx) {
      const url = `${baseUrl}${path}${query ? `?${query}` : ""}`;
      const { signal, deadline } = resolveContext(ctx, now);
      return serialize(() => execute(url, deadline, signal), deadline, signal);
    },
    redact,
  };
}

type BoardEnv = Pick<Env, "BOARD_BASE_URL" | "BOARD_API_KEY" | "BOARD_API_TOKEN">;

/** isolate 内で共有するクライアント。設定(キー・トークン含む)は値のまま保持して比較し、Map のキーやログには出さない。 */
const MAX_CACHED_CLIENTS = 8;
let clientCache: Array<{ env: BoardEnv; client: BoardClient }> = [];

/** テスト用: 共有しているクライアントを捨てる。 */
export function clearBoardClientCache(): void {
  clientCache = [];
}

/**
 * Worker の環境変数・secret からクライアントを作る。
 * 同じ設定なら isolate 内で同じインスタンスを返す(リクエストをまたいで直列化を効かせるため)。
 * `overrides` を渡したときは共有せず、新しく作る。
 */
export function createBoardClientFromEnv(
  env: BoardEnv,
  overrides: Partial<BoardClientOptions> = {},
): BoardClient {
  // `wrangler secret put` 経由などで末尾に改行が付いた値でも動くよう、前後の空白を除いて使う(checkConfig と同じ基準)。
  const baseUrl = env.BOARD_BASE_URL.trim();
  const apiKey = env.BOARD_API_KEY.trim();
  const apiToken = env.BOARD_API_TOKEN.trim();
  const create = () => createBoardClient({ baseUrl, apiKey, apiToken, ...overrides });
  if (Object.keys(overrides).length > 0) return create();
  const hit = clientCache.find(
    (e) =>
      e.env.BOARD_BASE_URL === baseUrl &&
      e.env.BOARD_API_KEY === apiKey &&
      e.env.BOARD_API_TOKEN === apiToken,
  );
  if (hit) return hit.client;
  const client = create();
  clientCache.push({
    env: {
      BOARD_BASE_URL: baseUrl,
      BOARD_API_KEY: apiKey,
      BOARD_API_TOKEN: apiToken,
    },
    client,
  });
  if (clientCache.length > MAX_CACHED_CLIENTS) clientCache.shift();
  return client;
}
