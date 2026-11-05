/**
 * refresh(Go 版 `internal/refresh/resource_refresh.go` `force_refresh.go` `updater.go` の移植 + Workers 向けの再開)。
 *
 * - delta: `updated_at_gteq = カーソル` で差分を取って upsert し、カーソルを取得分の updated_at の最大値に進める(Go と同じ)。
 *   カーソルがなければ全件が必要なので、再開できる full として動く。
 * - full: 世代を払い出してページ単位で upsert し、締め切りが近づいたら進捗を保存して partial で返す(Go との相違点)。
 *   次の full は続きのページから再開し、最後まで取り終えたら古い世代の行を消して complete にする。
 * - 排他: D1 の条件付き UPDATE(TryLock)。取れなければ待たずに RefreshInProgressError。成功・失敗・例外のどれでも finally で解放する。
 * - 秘密情報: sync_state に保存するのはエラーコードだけ。API キー・トークンはここを通らない。
 */

import type { BoardClient } from "../board/client";
import { type CallContext, resolveContext } from "../board/context";
import { isDeadlineError } from "../board/errors";
import { DEFAULT_PER_PAGE } from "../board/paging";
import { deleteOlderGenerationsStatement, upsertStatements } from "../cache/resource-cache";
import { ensureSchema } from "../cache/schema";
import { ensureRowStatement, getSyncState, markError, type SyncState } from "../cache/sync-state";
import { type Clock, isoSeconds, systemClock } from "../cache/time";
import { BoardError, RefreshInProgressError } from "../errors";
import { withLock } from "./lock";

/** 続きから再開できる partial の最長の古さ。これを過ぎたらページのずれが無視できないので、最初からやり直す。 */
export const FULL_RESUME_WINDOW_MS = 60 * 60_000;
/** full の開始時刻からこの分だけ巻き戻した時刻を、完了後のカーソルの上限にする(BOARD との時計のずれ・full 中の更新の保険)。 */
export const CURSOR_SAFETY_MARGIN_MS = 5 * 60_000;
/** 次のページを取りに行くのに最低限必要な残り時間の既定。直前のページの所要時間の 1.5 倍とのうち大きいほう。 */
export const DEFAULT_PAGE_BUDGET_FLOOR_MS = 1_000;

export interface RefreshTarget {
  resource: string;
  /** `/v1/clients` など。 */
  path: string;
  /** リソースの `build*Query` と同じ形。refresh は `updatedAtGteq` だけを渡す。 */
  buildQuery: (opts: { updatedAtGteq?: string }, page: number, perPage: number) => string;
}

export type RefreshMode = "delta" | "full";
export type RefreshState = "complete" | "partial" | "failed";

export interface RefreshOutcome {
  resource: string;
  /** 実際に動いたモード。カーソルのない delta は "full"。 */
  mode: RefreshMode;
  state: RefreshState;
  /** この呼び出しで取って書き込んだ件数。 */
  fetched: number;
  pages: number;
  /** partial(full)のとき、次に取るページ。 */
  nextPage?: number;
  /** failed(delta)のときのエラーコード。メッセージは載せない。 */
  errorCode?: string;
}

export interface RefreshDeps {
  db: D1Database;
  client: BoardClient;
  now?: Clock;
  /** ロックの取得者の識別子(テスト用)。 */
  owner?: string;
  /** 次のページに必要な残り時間の下限(テスト用)。 */
  pageBudgetFloorMs?: number;
  perPage?: number;
}

const OFFSET_RE = /^(\d{4}-\d{2}-\d{2})([T ])(\d{2}:\d{2}:\d{2})(\.\d+)?(Z|[+-]\d{2}:?\d{2})?$/;

/**
 * 保存してある ISO 8601 の updated_at を、BOARD の Ransack `_gteq` が期待する `YYYY-MM-DD HH:MM:SS` にする
 * (Go の isoToBoardDateTime)。元のオフセットの時刻部分をそのまま使う(UTC に直さない)。空は空、解釈できない値はそのまま返す。
 */
export function isoToBoardDateTime(s: string): string {
  if (s === "") return "";
  const m = OFFSET_RE.exec(s);
  return m ? `${m[1]} ${m[3]}` : s;
}

function offsetMinutes(offset: string | undefined): number {
  if (!offset || offset === "Z") return 0;
  const m = /^([+-])(\d{2}):?(\d{2})$/.exec(offset);
  if (!m) return 0;
  return (m[1] === "-" ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3]));
}

/**
 * full の完了後の delta カーソル。見た updated_at の最大値を基本とするが、それが full の開始の 5 分前より新しいときは
 * 開始の 5 分前(最大値と同じオフセットで表す)にする。full は数分かかるので、ページを取った後・最大値より前に起きた
 * 更新を取りこぼさないため(Go は full のあとカーソルを NULL に戻し、次の delta で全件を取り直す。それは締め切りの中では無理)。
 */
export function cursorAfterFull(maxUpdatedAt: string | null, startedAtIso: string): string | null {
  if (!maxUpdatedAt) return null;
  const m = OFFSET_RE.exec(maxUpdatedAt);
  if (!m) return maxUpdatedAt;
  const startMs = Date.parse(startedAtIso);
  if (Number.isNaN(startMs)) return maxUpdatedAt;
  const shifted = new Date(startMs - CURSOR_SAFETY_MARGIN_MS + offsetMinutes(m[5]) * 60_000);
  const local = shifted
    .toISOString()
    .replace(/\.\d{3}Z$/, "")
    .replace("T", m[2] ?? "T");
  const candidate = `${local}${m[4] ? ".000" : ""}${m[5] ?? ""}`;
  return candidate < maxUpdatedAt ? candidate : maxUpdatedAt;
}

function maxString(a: string | null, b: string | null): string | null {
  if (a === null) return b;
  if (b === null) return a;
  return b > a ? b : a;
}

function maxUpdatedAt(items: readonly unknown[]): string | null {
  let max: string | null = null;
  for (const e of items) {
    const v = (e as { updated_at?: unknown } | null)?.updated_at;
    if (typeof v === "string" && v !== "") max = maxString(max, v);
  }
  return max;
}

interface Run {
  deps: RefreshDeps;
  target: RefreshTarget;
  ctx: CallContext & { deadline: number };
  now: Clock;
  perPage: number;
  floorMs: number;
}

async function fetchPage(run: Run, query: string): Promise<unknown[]> {
  const res = await run.deps.client.get(run.target.path, query, run.ctx);
  if (!Array.isArray(res.body)) {
    throw new BoardError(
      "api_error",
      "BOARD API returned an unexpected response (expected a JSON array).",
      false,
    );
  }
  return res.body;
}

/** 次のページを取る余裕があるか。直前のページの所要時間の 1.5 倍(下限あり)を残り時間と比べる。 */
function hasTimeForNextPage(run: Run, lastPageMs: number): boolean {
  return run.ctx.deadline - run.now() >= Math.max(run.floorMs, lastPageMs * 1.5);
}

/**
 * 締め切りで止まった失敗か。クライアントが締め切りの型のエラーを返す場合に加え、最後の 1 試行が『残り時間』で
 * タイムアウトした場合(通信失敗として返る)も、締め切りが過ぎていれば同じ扱いにする。
 */
function isOutOfTime(run: Run, e: unknown): boolean {
  if (run.ctx.signal?.aborted) return false;
  if (isDeadlineError(e)) return true;
  return e instanceof BoardError && e.retryable && run.ctx.deadline - run.now() <= TIMER_SLACK_MS;
}

/** タイマーが締め切りより少し早く発火しても『締め切り』と判定するための余裕。 */
const TIMER_SLACK_MS = 50;

const errorCodeOf = (e: unknown): string => (e instanceof BoardError ? e.code : "generic_error");

/** 想定内の失敗を sync_state に記録する。呼び出し元の中断(signal)は失敗として数えない。 */
async function recordFailure(run: Run, e: unknown): Promise<void> {
  if (run.ctx.signal?.aborted) return;
  await markError(run.deps.db, run.target.resource, errorCodeOf(e), isoSeconds(run.now()));
}

async function runFull(run: Run, state: SyncState | null): Promise<RefreshOutcome> {
  const { db } = run.deps;
  const resource = run.target.resource;
  const nowMs = run.now();

  const startedMs = state?.full_started_at ? Date.parse(state.full_started_at) : Number.NaN;
  const resumable =
    state?.full_next_page != null &&
    !Number.isNaN(startedMs) &&
    nowMs - startedMs <= FULL_RESUME_WINDOW_MS;

  let generation: number;
  let page: number;
  let startedAt: string;
  let maxSeen: string | null;
  if (resumable && state) {
    generation = state.full_generation;
    page = state.full_next_page ?? 1;
    startedAt = state.full_started_at ?? isoSeconds(nowMs);
    maxSeen = state.full_max_updated_at;
  } else {
    generation = (state?.full_generation ?? 0) + 1;
    page = 1;
    startedAt = isoSeconds(nowMs);
    maxSeen = null;
    // 世代の払い出しを先に記録する(最初のページが失敗しても、次はこの世代の続きから始められる)。
    await db.batch([
      ensureRowStatement(db, resource),
      db
        .prepare(
          `UPDATE sync_state SET full_generation = ?, full_next_page = 1, full_started_at = ?,
           full_max_updated_at = NULL, last_sync_mode = 'full', last_sync_status = 'partial'
           WHERE resource_name = ?`,
        )
        .bind(generation, startedAt, resource),
    ]);
  }

  let fetched = 0;
  let pages = 0;
  let lastPageMs = 0;
  for (;;) {
    if (!hasTimeForNextPage(run, lastPageMs)) {
      return { resource, mode: "full", state: "partial", fetched, pages, nextPage: page };
    }
    const t0 = run.now();
    let items: unknown[];
    try {
      items = await fetchPage(run, run.target.buildQuery({}, page, run.perPage));
    } catch (e) {
      if (isOutOfTime(run, e)) {
        return { resource, mode: "full", state: "partial", fetched, pages, nextPage: page };
      }
      await recordFailure(run, e);
      throw e;
    }

    const isLast = items.length < run.perPage;
    try {
      maxSeen = maxString(maxSeen, maxUpdatedAt(items));
      const at = run.now();
      const statements = upsertStatements(db, resource, items, { generation, now: at });
      if (isLast) {
        statements.push(
          deleteOlderGenerationsStatement(db, resource, generation),
          db
            .prepare(
              `UPDATE sync_state SET last_synced_at = ?, last_full_synced_at = ?, last_sync_mode = 'full',
               last_sync_status = 'success', cursor_updated_at = ?, must_full_resync = 0,
               consecutive_failures = 0, full_next_page = NULL, full_max_updated_at = NULL
               WHERE resource_name = ?`,
            )
            .bind(isoSeconds(at), isoSeconds(at), cursorAfterFull(maxSeen, startedAt), resource),
        );
      } else {
        statements.push(
          db
            .prepare(
              "UPDATE sync_state SET full_next_page = ?, full_max_updated_at = ? WHERE resource_name = ?",
            )
            .bind(page + 1, maxSeen, resource),
        );
      }
      await db.batch(statements);
    } catch (e) {
      await recordFailure(run, e);
      throw e;
    }
    fetched += items.length;
    pages++;
    if (isLast) return { resource, mode: "full", state: "complete", fetched, pages };
    page++;
    lastPageMs = run.now() - t0;
  }
}

async function runDelta(run: Run, state: SyncState): Promise<RefreshOutcome> {
  const { db } = run.deps;
  const resource = run.target.resource;
  const gteq = isoToBoardDateTime(state.cursor_updated_at ?? "");
  // delta の書き込みは進行中(または最新)の世代を付ける。full の完了時に消されないように。
  const generation = state.full_generation;

  let fetched = 0;
  let pages = 0;
  let lastPageMs = 0;
  let maxSeen: string | null = null;
  for (let page = 1; ; page++) {
    if (!hasTimeForNextPage(run, lastPageMs)) {
      return { resource, mode: "delta", state: "partial", fetched, pages };
    }
    const t0 = run.now();
    let items: unknown[];
    try {
      items = await fetchPage(
        run,
        run.target.buildQuery({ updatedAtGteq: gteq }, page, run.perPage),
      );
      maxSeen = maxString(maxSeen, maxUpdatedAt(items));
      const at = run.now();
      const statements = upsertStatements(db, resource, items, { generation, now: at });
      const isLast = items.length < run.perPage;
      if (isLast) {
        statements.push(
          db
            .prepare(
              `UPDATE sync_state SET last_synced_at = ?, last_sync_mode = 'delta', last_sync_status = 'success',
               cursor_updated_at = COALESCE(?, cursor_updated_at), consecutive_failures = 0
               WHERE resource_name = ?`,
            )
            .bind(isoSeconds(at), maxSeen, resource),
        );
      }
      if (statements.length > 0) await db.batch(statements);
      fetched += items.length;
      pages++;
      if (isLast) return { resource, mode: "delta", state: "complete", fetched, pages };
    } catch (e) {
      if (isOutOfTime(run, e)) return { resource, mode: "delta", state: "partial", fetched, pages };
      if (run.ctx.signal?.aborted) throw e;
      // Go と同じ: delta の失敗は例外にせず、古いキャッシュを返せるようにする(失敗は記録し、呼び出し元に state で伝える)。
      await recordFailure(run, e);
      return {
        resource,
        mode: "delta",
        state: "failed",
        fetched,
        pages,
        errorCode: errorCodeOf(e),
      };
    }
    lastPageMs = run.now() - t0;
  }
}

/**
 * 1 リソースの refresh。ロックを取って(取れなければ RefreshInProgressError)、delta または full を実行し、必ず解放する。
 * `ctx` の締め切り・signal に従う。省略時は呼び出し開始から DEFAULT_CALL_BUDGET_MS。
 */
export async function refreshResource(
  deps: RefreshDeps,
  target: RefreshTarget,
  mode: RefreshMode,
  ctx?: CallContext,
): Promise<RefreshOutcome> {
  const now = deps.now ?? systemClock;
  const run: Run = {
    deps,
    target,
    ctx: resolveContext(ctx, now),
    now,
    perPage: deps.perPage ?? DEFAULT_PER_PAGE,
    floorMs: deps.pageBudgetFloorMs ?? DEFAULT_PAGE_BUDGET_FLOOR_MS,
  };
  await ensureSchema(deps.db);
  return withLock(deps.db, target.resource, { now, owner: deps.owner }, async () => {
    const state = await getSyncState(deps.db, target.resource);
    if (mode === "delta" && state?.cursor_updated_at) return runDelta(run, state);
    return runFull(run, state);
  });
}

export { RefreshInProgressError };
