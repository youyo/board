import { createMcpHandler } from "agents/mcp/server";
import { type Context, Hono } from "hono";
import { type AccessIdentity, AccessJwtError, verifyAccessJwt } from "./auth/access-jwt";
import type { Env } from "./env";
import { logEvent, requestIdOf } from "./log";
import { createMcpServer } from "./mcp/server";

type AppEnv = { Bindings: Env; Variables: { access: AccessIdentity; requestId: string } };

export const MCP_PATH = "/mcp";

/**
 * Worker 全体の Hono アプリ。設定チェック(config-check.ts)は src/index.ts がこの前に行う。
 * 認可は Cloudflare Access に任せ(Managed OAuth)、Worker は `/healthz` 以外の全リクエストで
 * Cf-Access-Jwt-Assertion を検証する。
 */
export const app = new Hono<AppEnv>();

// /healthz は認証なし。ミドルウェアより前に登録して、検証の対象から外す。
app.get("/healthz", (c) => c.json({ status: "ok" }, 200, { "Cache-Control": "no-store" }));

app.use("*", accessGuard);

async function accessGuard(
  c: Context<AppEnv>,
  next: () => Promise<void>,
): Promise<Response | undefined> {
  const requestId = requestIdOf(c.req.raw);
  c.set("requestId", requestId);
  try {
    const identity = await verifyAccessJwt(c.req.header("Cf-Access-Jwt-Assertion"), {
      teamDomain: c.env.ACCESS_TEAM_DOMAIN,
      aud: c.env.ACCESS_AUD,
    });
    c.set("access", identity);
  } catch (e) {
    // 理由の種別だけを残す。応答にもログにも JWT・鍵は出さない。
    const reason = e instanceof AccessJwtError ? e.reason : "error";
    logEvent("warn", "access_jwt_rejected", requestId, { reason });
    const headers = { "Cache-Control": "no-store" };
    return reason === "missing"
      ? c.json({ error: "unauthorized" }, 401, headers)
      : c.json({ error: "forbidden" }, 403, headers);
  }
  await next();
  return undefined;
}

/** ログに残す主体。人は email、service token は Client ID(sub が空のため)。JWT 自体は渡さない。 */
export function actorOf(identity: AccessIdentity): string {
  return identity.kind === "user" ? identity.email || identity.subject : identity.clientId;
}

/**
 * ブラウザからの呼び出しは想定しないので、ライブラリの既定値に頼らず明示する(将来既定値が変わっても影響を受けない)。
 * - `corsOptions: false`: CORS ヘッダーを付けない(`Access-Control-*` は応答から取り除かれる)。
 * - `allowedOriginHostnames: []`: Origin ヘッダーのある要求はすべて 403。Origin のない MCP クライアントだけを通す。
 *   空配列は `undefined`(= localhost・workers.dev を許す既定値)とは別物で、既定値には fallback しない
 *   (node_modules/agents/dist/handler-stateless-*.js の `acceptedOriginHostnames === void 0` のときだけ既定値を作る)。
 */
export const MCP_HANDLER_OPTIONS = {
  route: MCP_PATH,
  corsOptions: false,
  allowedOriginHostnames: [],
} as const satisfies Parameters<typeof createMcpHandler>[1];

// リクエストごとに McpServer を作る(ステートレス。Durable Objects もセッションも使わない)。
// Hono の c.executionCtx の型は tracing / abort を持たない。実体は Workers の ctx そのもの。
app.all(MCP_PATH, (c) =>
  createMcpHandler(
    () =>
      createMcpServer({
        env: c.env,
        requestId: c.get("requestId"),
        actor: actorOf(c.get("access")),
      }),
    MCP_HANDLER_OPTIONS,
  )(c.req.raw, c.env, c.executionCtx as unknown as ExecutionContext<unknown>),
);
