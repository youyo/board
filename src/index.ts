import { app } from "./app";
import { checkConfig } from "./config-check";
import type { Env } from "./env";
import { logEvent, requestIdOf } from "./log";

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    // 必須の設定が欠けた Worker は、/healthz 以外を 500 にする(logbutler の ADR 0010 と同じ)。
    const config = checkConfig(env as unknown as Record<string, unknown>);
    if (!config.ok) {
      if (new URL(request.url).pathname === "/healthz") {
        return Response.json(
          { status: "misconfigured", missing: config.missing, invalid: config.invalid },
          { status: 503, headers: { "Cache-Control": "no-store" } },
        );
      }
      // 詳細(項目名だけ。値は出さない)はログへ。応答には出さない。
      logEvent("error", "config_invalid", requestIdOf(request), {
        missing: config.missing,
        invalid: config.invalid,
      });
      return Response.json(
        { error: "server_misconfigured" },
        { status: 500, headers: { "Cache-Control": "no-store" } },
      );
    }
    return app.fetch(request, env, ctx);
  },
} satisfies ExportedHandler<Env>;
