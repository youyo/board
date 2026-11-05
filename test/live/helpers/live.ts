/**
 * 実環境(デプロイ済み Worker + 実 BOARD)に対する確認のクライアント(`mise run test:live`)。
 *
 * - 認証は Cloudflare Access の service token。ヘッダー `CF-Access-Client-Id` / `CF-Access-Client-Secret` で送ると、
 *   Access が JWT(`Cf-Access-Jwt-Assertion`)に変換して Worker に渡す。
 * - 呼び出しは直列(BOARD の rate limit のため)。応答時間とサイズを返す。
 * - service token の値は出力・エラーメッセージに出さない。
 * - 状態を持たない MCP(createMcpHandler)なので initialize なしで tools/call を送る。
 */

export interface LiveConfig {
  /** `https://<domain>/mcp` */
  url: string;
  clientId: string;
  clientSecret: string;
}

export type LoadResult = { ok: true; config: LiveConfig } | { ok: false; reason: string };

const REQUEST_TIMEOUT_MS = 60_000;

/** 環境変数から設定を読む。足りなければ、変数名を挙げた skip の理由を返す(値は載せない)。 */
export function loadLiveConfig(env: Record<string, string | undefined>): LoadResult {
  const names = [
    "BOARD_LIVE_URL",
    "BOARD_LIVE_ACCESS_CLIENT_ID",
    "BOARD_LIVE_ACCESS_CLIENT_SECRET",
  ] as const;
  const missing = names.filter((n) => !env[n]?.trim());
  if (missing.length > 0) {
    return {
      ok: false,
      reason: `未設定の環境変数: ${missing.join(", ")}(デプロイ済み Worker の URL と Access の service token が必要。docs/runbook.md の『live 用の service token』)`,
    };
  }
  const url = (env.BOARD_LIVE_URL as string).trim();
  let valid = false;
  try {
    const u = new URL(url);
    valid = u.protocol === "https:" && u.pathname === "/mcp" && u.search === "" && u.hash === "";
  } catch {
    // 下で報告する
  }
  if (!valid) {
    return {
      ok: false,
      reason:
        "BOARD_LIVE_URL は https://<domain>/mcp の形にする(service token を平文で送らないため https のみ)",
    };
  }
  return {
    ok: true,
    config: {
      url,
      clientId: (env.BOARD_LIVE_ACCESS_CLIENT_ID as string).trim(),
      clientSecret: (env.BOARD_LIVE_ACCESS_CLIENT_SECRET as string).trim(),
    },
  };
}

/** 応答は JSON または SSE(`data: {...}` 行)のどちらでも来る。 */
export function parseRpcBody(text: string): unknown {
  const data = text.trimStart().startsWith("{")
    ? text
    : text
        .split("\n")
        .find((l) => l.startsWith("data:"))
        ?.slice(5)
        .trim();
  return JSON.parse(data ?? "null");
}

export function summarize(values: readonly number[]) {
  if (values.length === 0) return { n: 0, total: 0, min: 0, median: 0, max: 0 };
  const sorted = [...values].sort((a, b) => a - b);
  return {
    n: sorted.length,
    total: sorted.reduce((a, b) => a + b, 0),
    min: sorted[0] as number,
    median: sorted[Math.floor(sorted.length / 2)] as number,
    max: sorted[sorted.length - 1] as number,
  };
}

export interface ToolCall {
  isError: boolean;
  text: string;
  /** text を JSON として読んだもの。読めなければ undefined。 */
  // biome-ignore lint/suspicious/noExplicitAny: 応答の形はテストごとに検証する
  json: any;
  /** 応答本文のバイト数(HTTP の本文。MCP の包みを含む)。 */
  bytes: number;
  /** リクエストの送信から本文を読み終えるまで(ミリ秒)。 */
  ms: number;
}

export class LiveClient {
  /** Worker への HTTP リクエスト数。 */
  requests = 0;
  private nextId = 1;

  constructor(
    private readonly config: LiveConfig,
    private readonly fetchImpl: typeof fetch = fetch,
  ) {}

  private redact(s: string): string {
    return s
      .replaceAll(this.config.clientSecret, "[REDACTED]")
      .replaceAll(this.config.clientId, "[REDACTED]");
  }

  async call(name: string, args: Record<string, unknown> = {}): Promise<ToolCall> {
    this.requests++;
    const started = performance.now();
    let res: Response;
    let text: string;
    try {
      res = await this.fetchImpl(this.config.url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json, text/event-stream",
          "CF-Access-Client-Id": this.config.clientId,
          "CF-Access-Client-Secret": this.config.clientSecret,
        },
        body: JSON.stringify({
          jsonrpc: "2.0",
          id: this.nextId++,
          method: "tools/call",
          params: { name, arguments: args },
        }),
        signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
      });
      text = await res.text();
    } catch (e) {
      throw new Error(
        this.redact(`${name}: リクエストに失敗: ${e instanceof Error ? e.message : String(e)}`),
      );
    }
    const ms = performance.now() - started;
    if (res.status !== 200) {
      // 401/403 は Access か Worker の JWT 検証で弾かれている(service token と Access ポリシーを確認する)。
      throw new Error(this.redact(`${name}: HTTP ${res.status}: ${text.slice(0, 200)}`));
    }
    // biome-ignore lint/suspicious/noExplicitAny: JSON-RPC 応答
    const body = parseRpcBody(text) as any;
    if (body?.error) {
      throw new Error(
        this.redact(`${name}: JSON-RPC エラー: ${JSON.stringify(body.error).slice(0, 200)}`),
      );
    }
    const payload: string = body?.result?.content?.[0]?.text ?? "";
    let json: unknown;
    try {
      json = JSON.parse(payload);
    } catch {
      json = undefined;
    }
    return {
      isError: body?.result?.isError === true,
      text: payload,
      json,
      bytes: new TextEncoder().encode(text).length,
      ms,
    };
  }
}
