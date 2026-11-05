import { type CallContext, DEFAULT_CALL_BUDGET_MS } from "../board/context";
import type { ToolContext } from "./registry";

/**
 * ツール呼び出しごとに 1 つ作る。signal は MCP のリクエストのもの。
 * 締め切りは `Date.now()` を基準にする: BOARD クライアントの残り時間の判定が `Date.now()` なので、
 * ツールの時計(`ctx.now()`。テストで差し替える)から作ると、両者がずれて締め切りの判定が食い違う(M04 の指摘)。
 */
export function callContextOf(ctx: ToolContext): CallContext {
  return { signal: ctx.signal, deadline: Date.now() + DEFAULT_CALL_BUDGET_MS };
}
