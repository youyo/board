/**
 * 呼び出し単位のコンテキスト。ツール呼び出しごとに 1 つ作り、`get` / 一覧 / 取得に渡す。
 * 一覧のページ巡回など、1 回の呼び出しの中の通信は同じ締め切りを共有する。
 */
export interface CallContext {
  /** 呼び出し元(MCP クライアント)が切れたら abort される。fetch・待機・キュー待ちを中断する。 */
  signal?: AbortSignal;
  /** 締め切り(epoch ms)。省略時は呼び出し開始から DEFAULT_CALL_BUDGET_MS。 */
  deadline?: number;
}

/** deadline を省いたときの呼び出し全体の持ち時間(ミリ秒)。 */
export const DEFAULT_CALL_BUDGET_MS = 30_000;

/** deadline が未指定なら `now + DEFAULT_CALL_BUDGET_MS` で埋めたコンテキストを返す。 */
export function resolveContext(
  ctx: CallContext | undefined,
  now: () => number = Date.now,
): CallContext & { deadline: number } {
  return { signal: ctx?.signal, deadline: ctx?.deadline ?? now() + DEFAULT_CALL_BUDGET_MS };
}
