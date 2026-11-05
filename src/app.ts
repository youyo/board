import { createMcpHandler } from "agents/mcp/server";
import { type Context, Hono } from "hono";
import { type AccessIdentity, AccessJwtError, verifyAccessJwt } from "./auth/access-jwt";
import type { Env } from "./env";
import { createMcpServer } from "./mcp/server";

type AppEnv = { Bindings: Env; Variables: { access: AccessIdentity } };

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
  try {
    const identity = await verifyAccessJwt(c.req.header("Cf-Access-Jwt-Assertion"), {
      teamDomain: c.env.ACCESS_TEAM_DOMAIN,
      aud: c.env.ACCESS_AUD,
    });
    c.set("access", identity);
  } catch (e) {
    // 理由の種別だけを残す。応答にもログにも JWT・鍵は出さない。
    const reason = e instanceof AccessJwtError ? e.reason : "error";
    console.warn(`access jwt rejected: ${reason}`);
    const headers = { "Cache-Control": "no-store" };
    return reason === "missing"
      ? c.json({ error: "unauthorized" }, 401, headers)
      : c.json({ error: "forbidden" }, 403, headers);
  }
  await next();
  return undefined;
}

// リクエストごとに McpServer を作る(ステートレス。Durable Objects もセッションも使わない)。
// Hono の c.executionCtx の型は tracing / abort を持たない。実体は Workers の ctx そのもの。
app.all(MCP_PATH, (c) =>
  createMcpHandler(() => createMcpServer(), { route: MCP_PATH })(
    c.req.raw,
    c.env,
    c.executionCtx as unknown as ExecutionContext<unknown>,
  ),
);
