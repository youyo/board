/**
 * refresh(Go 版 `internal/refresh/resource_refresh.go` `force_refresh.go` `updater.go` の移植 + Workers 向けの再開)。
 *
 * - delta: `updated_at_gteq = カーソル` で差分を取って upsert する。full と同じ方式で再開できる(Go との相違点): ページごとに進捗
 *   (delta_next_page / delta_started_at / delta_max_updated_at)を保存し、締め切りで partial になったら次の delta は最後に取り終えた
 *   ページから(1 ページ重ねて)取り直す。最後まで取ったらカーソルを min(見た updated_at の最大値, 開始の 5 分前)に進める(`cursorAfterFull` と同じ)。
 *   再開できる窓は full と同じ 1 時間。
 *   カーソルがなければ全件が必要なので、再開できる full として動く。仕様の一覧に `updated_at_gteq` が無いリソース
 *   (users / groups / payment_terms / 3 つの区分 / document_send_channels / project_costs)も同じ分岐で full として動く(`mode: "full"` を返す。issue #28)。
 * - full: 世代を払い出してページ単位で upsert し、締め切りが近づいたら進捗を保存して partial で返す(Go との相違点)。
 *   次の full は最後に取り終えたページから(1 ページ重ねて)再開し、最後まで取り終えたら古い世代の行を消して complete にする。
 *   消す前に BOARD の総件数(X-Total-Count)と新しい世代の件数を照合し、足りなければ消さずに `full_prune_skipped` を記録する。
 *   ページ番号での再開は BOARD の一覧の既定の並びが呼び出しの間で安定していることを前提にしている(詳細は計画書)。
 * - 排他: D1 の条件付き UPDATE(TryLock)。取れなければ待たずに RefreshInProgressError。成功・失敗・例外のどれでも finally で解放する。
 *   書き込み(行の upsert・旧世代の削除・sync_state の更新)は、すべて同じ D1 batch の中で `refresh_owner` が自分であることを条件にする。
 *   stale ロックを奪われた元の保持者は何も書けず、sync_state の更新の `meta.changes` が 0 になるので RefreshInProgressError で止まる。
 * - 秘密情報: sync_state に保存するのはエラーコードだけ。API キー・トークンはここを通らない。
 */

import type { BoardClient } from "../board/client";
import { type CallContext, resolveContext } from "../board/context";
import { isDeadlineError } from "../board/errors";
import { parseListMeta } from "../board/meta";
import { LIST_FILTER_SUPPORT } from "../board/openapi-support";
import { DEFAULT_PER_PAGE } from "../board/paging";
import {
  deleteOlderGenerationsStatement,
  PRUNE_OK_SQL,
  upsertStatements,
} from "../cache/resource-cache";
import { ensureSchema } from "../cache/schema";
import {
  CACHE_QUERY_VERSION,
  deltaInProgress,
  ensureRowStatement,
  fullInProgress,
  getSyncState,
  markError,
  queryVersionCurrent,
  type SyncState,
} from "../cache/sync-state";
import { type Clock, isoSeconds, systemClock } from "../cache/time";
import { BoardError, RefreshInProgressError } from "../errors";
import { withLock } from "./lock";

/** 一覧が updated_at_gteq を受けるか(OpenAPI から生成した `LIST_FILTER_SUPPORT`)。未知のリソースは受けるものとして扱う。 */
const supportsDelta = (resource: string): boolean =>
  LIST_FILTER_SUPPORT[resource]?.updatedAtGteq ?? true;

/** 続きから再開できる partial の最長の古さ。これを過ぎたらページのずれが無視できないので、最初からやり直す。 */
export const FULL_RESUME_WINDOW_MS = 60 * 60_000;
/**
 * must_full_resync=1 を理由に refresh=delta を full へ切り替える最短の間隔(前回の full の完了から)。
 * BOARD の X-Total-Count が恒常的に多いと毎回 full になり、日次の rate limit を使い切るため(issue #40)。
 */
export const MUST_FULL_RESYNC_MIN_INTERVAL_MS = 6 * 60 * 60_000;
/** full の開始時刻からこの分だけ巻き戻した時刻を、完了後のカーソルの上限にする(BOARD との時計のずれ・full 中の更新の保険)。 */
export const CURSOR_SAFETY_MARGIN_MS = 5 * 60_000;
/** 次のページを取りに行くのに最低限必要な残り時間の既定。直前のページの所要時間の 1.5 倍とのうち大きいほう。 */
export const DEFAULT_PAGE_BUDGET_FLOOR_MS = 1_000;

export interface RefreshTarget {
  resource: string;
  /** `/v1/clients` など。 */
  path: string;
  /**
   * リソースの `build*Query` と同じ形。refresh は `updatedAtGteq` と、アーカイブ・失注を含めるフラグ(REFRESH_INCLUDE_OPTS)を渡す。
   * フラグを受けないリソースには `baseQuery` が送らない。`includeLostFlg` は projects の `buildProjectsQuery` だけが読む。
   */
  buildQuery: (
    opts: { updatedAtGteq?: string; includeArchiveFlg?: boolean; includeLostFlg?: boolean },
    page: number,
    perPage: number,
  ) => string;
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
  /** partial(full)のとき、次に取るページ(再開は 1 つ前のページから重ねて取り直す)。 */
  nextPage?: number;
  /** full の完了時に、件数の照合に通らず旧世代を消さなかった。 */
  pruneSkipped?: boolean;
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

/**
 * refresh は BOARD の既定(アーカイブ済み・失注を除外)ではなく、すべてを取ってキャッシュに入れる(issue #35)。
 * full でも delta でも送る(送らないと、前回の同期のあとにアーカイブ・失注になったレコードの更新を取りこぼす)。
 * 一覧の X-Total-Count も同じ条件の件数になるので、full の prune の照合はそのまま成り立つ。
 */
export const REFRESH_INCLUDE_OPTS = { includeArchiveFlg: true, includeLostFlg: true } as const;

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

function isValidUpdatedAt(s: string): boolean {
  const m = OFFSET_RE.exec(s);
  if (!m) return false;
  // 月日・時刻の範囲まで確かめる(正規表現だけでは 2026-13-45 を通してしまう)。UTC として組み直して同じ値になること。
  const iso = `${m[1]}T${m[3]}Z`;
  const t = Date.parse(iso);
  return !Number.isNaN(t) && new Date(t).toISOString().startsWith(`${m[1]}T${m[3]}`);
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

/** 形式が正しい(ISO 8601 の日時として解釈できる)updated_at だけを最大値の計算に使う。解釈できない値は無視する。 */
function maxUpdatedAt(items: readonly unknown[]): string | null {
  let max: string | null = null;
  for (const e of items) {
    const v = (e as { updated_at?: unknown } | null)?.updated_at;
    if (typeof v === "string" && isValidUpdatedAt(v)) max = maxString(max, v);
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
  /** 保持しているロックの所有者。書き込みはすべてこの所有者を条件にする。 */
  owner: string;
}

async function fetchPage(
  run: Run,
  query: string,
): Promise<{ items: unknown[]; total: number | null }> {
  const res = await run.deps.client.get(run.target.path, query, run.ctx);
  if (!Array.isArray(res.body)) {
    throw new BoardError(
      "api_error",
      "BOARD API returned an unexpected response (expected a JSON array).",
      false,
    );
  }
  return { items: res.body, total: parseListMeta(res.headers).total_count ?? null };
}

/** ロックを失った(stale として奪われた)ことを表す。呼び出し元には refresh_in_progress として伝わる。 */
function lostLock(run: Run): RefreshInProgressError {
  return new RefreshInProgressError(run.target.resource);
}

/** 所有者を条件にした sync_state の更新が空振りした(changes が 1 でない)ら、ロックを失ったとみなして止める。 */
function assertOwned(run: Run, result: D1Result | undefined): void {
  if (result?.meta.changes !== 1) throw lostLock(run);
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
  if (e instanceof RefreshInProgressError) return; // ロックを失った: 他の保持者の状態を汚さない。
  await markError(
    run.deps.db,
    run.target.resource,
    errorCodeOf(e),
    isoSeconds(run.now()),
    run.owner,
  );
}

/** must_full_resync による delta → full の切り替え。前回の full の完了から間隔が空いているときだけ(完了時刻が無い・読めないときは切り替える)。 */
function mustSwitchToFull(state: SyncState, nowMs: number): boolean {
  if (!state.must_full_resync) return false;
  const lastFullMs = state.last_full_synced_at ? Date.parse(state.last_full_synced_at) : Number.NaN;
  if (Number.isNaN(lastFullMs)) return true;
  return nowMs - lastFullMs >= MUST_FULL_RESYNC_MIN_INTERVAL_MS;
}

async function runFull(run: Run, state: SyncState | null): Promise<RefreshOutcome> {
  const { db } = run.deps;
  const resource = run.target.resource;
  const nowMs = run.now();

  const startedMs = state?.full_started_at ? Date.parse(state.full_started_at) : Number.NaN;
  // 旧コード(アーカイブ・失注を取らない)で保存した進捗は再開しない(版が違えば最初から)。
  const resumable =
    fullInProgress(state) && !Number.isNaN(startedMs) && nowMs - startedMs <= FULL_RESUME_WINDOW_MS;

  let generation: number;
  let page: number;
  let startedAt: string;
  let maxSeen: string | null;
  if (resumable && state) {
    generation = state.full_generation;
    // 最後に取り終えたページから取り直す(1 ページ重ねる)。BOARD 側で前のページの行が減ると、続きのページの先頭の行が
    // 1 つ前のページに繰り上がり、続きから取ると取りこぼして完了時に旧世代として消してしまうため。upsert は冪等なので重複は問題ない。
    page = Math.max(1, (state.full_next_page ?? 1) - 1);
    startedAt = state.full_started_at ?? isoSeconds(nowMs);
    maxSeen = state.full_max_updated_at;
  } else {
    generation = (state?.full_generation ?? 0) + 1;
    page = 1;
    startedAt = isoSeconds(nowMs);
    maxSeen = null;
    // 世代の払い出しを先に記録する(最初のページが失敗しても、次はこの世代の続きから始められる)。
    const started = await db.batch([
      ensureRowStatement(db, resource),
      db
        .prepare(
          `UPDATE sync_state SET full_generation = ?, full_next_page = 1, full_started_at = ?,
           full_max_updated_at = NULL, full_query_version = ?, last_sync_mode = 'full', last_sync_status = 'partial'
           WHERE resource_name = ? AND refresh_owner = ?`,
        )
        .bind(generation, startedAt, CACHE_QUERY_VERSION, resource, run.owner),
    ]);
    assertOwned(run, started[1]);
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
    let total: number | null;
    try {
      ({ items, total } = await fetchPage(
        run,
        run.target.buildQuery({ ...REFRESH_INCLUDE_OPTS }, page, run.perPage),
      ));
    } catch (e) {
      if (isOutOfTime(run, e)) {
        return { resource, mode: "full", state: "partial", fetched, pages, nextPage: page };
      }
      await recordFailure(run, e);
      throw e;
    }

    const isLast = items.length < run.perPage;
    let pruneSkipped = false;
    try {
      maxSeen = maxString(maxSeen, maxUpdatedAt(items));
      const at = run.now();
      const guardOwner = run.owner;
      const statements = upsertStatements(db, resource, items, { generation, now: at, guardOwner });
      if (isLast) {
        // 旧世代の削除と完了の記録は、新しい世代の件数が BOARD の総件数に届いているときだけ削除まで行う(届かなければ旧世代を残す)。
        statements.push(
          deleteOlderGenerationsStatement(db, resource, generation, {
            guardOwner,
            expectedTotal: total,
          }),
          db
            .prepare(
              `UPDATE sync_state SET last_synced_at = ?, last_full_synced_at = ?, last_sync_mode = 'full',
               last_sync_status = 'success', cursor_updated_at = ?,
               must_full_resync = CASE WHEN ${PRUNE_OK_SQL} THEN 0 ELSE 1 END,
               consecutive_failures = 0, full_next_page = NULL, full_max_updated_at = NULL,
               delta_next_page = NULL, delta_started_at = NULL, delta_max_updated_at = NULL,
               query_version = CASE WHEN ${PRUNE_OK_SQL} THEN ${CACHE_QUERY_VERSION} ELSE query_version END,
               full_query_version = NULL, delta_query_version = NULL,
               full_prune_skipped = CASE WHEN ${PRUNE_OK_SQL} THEN 0 ELSE 1 END
               WHERE resource_name = ? AND refresh_owner = ?`,
            )
            .bind(
              isoSeconds(at),
              isoSeconds(at),
              cursorAfterFull(maxSeen, startedAt),
              // プレースホルダは SQL の出現順に、先頭の ? 3 個(last_synced_at / last_full_synced_at / cursor)のあと
              // PRUNE_OK_SQL(? 4 個: total, resource, generation, total)が 3 回、最後に WHERE の 2 個(resource, owner)で、計 17 個。
              // must_full_resync の PRUNE_OK_SQL
              total,
              resource,
              generation,
              total,
              // query_version の PRUNE_OK_SQL
              total,
              resource,
              generation,
              total,
              // full_prune_skipped の PRUNE_OK_SQL
              total,
              resource,
              generation,
              total,
              resource,
              guardOwner,
            ),
        );
      } else {
        statements.push(
          db
            .prepare(
              "UPDATE sync_state SET full_next_page = ?, full_max_updated_at = ?, full_query_version = ? WHERE resource_name = ? AND refresh_owner = ?",
            )
            .bind(page + 1, maxSeen, CACHE_QUERY_VERSION, resource, guardOwner),
        );
      }
      const results = await db.batch(statements);
      assertOwned(run, results[results.length - 1]);
      if (isLast) pruneSkipped = (await getSyncState(db, resource))?.full_prune_skipped === 1;
    } catch (e) {
      await recordFailure(run, e);
      throw e;
    }
    fetched += items.length;
    pages++;
    if (isLast) {
      return {
        resource,
        mode: "full",
        state: "complete",
        fetched,
        pages,
        ...(pruneSkipped ? { pruneSkipped: true } : {}),
      };
    }
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

  // full と同じ方式で再開する。カーソルは完了するまで動かさないので、再開しても同じ gteq で取り直せる。
  const nowMs = run.now();
  const savedStartMs = state.delta_started_at ? Date.parse(state.delta_started_at) : Number.NaN;
  const resumable =
    deltaInProgress(state) &&
    !Number.isNaN(savedStartMs) &&
    nowMs - savedStartMs <= FULL_RESUME_WINDOW_MS;
  // 最後に取り終えたページから取り直す(1 ページ重ねる)。理由は full と同じ。
  let page = resumable ? Math.max(1, (state.delta_next_page ?? 1) - 1) : 1;
  const startedAt =
    resumable && state.delta_started_at ? state.delta_started_at : isoSeconds(nowMs);
  let maxSeen: string | null = resumable ? state.delta_max_updated_at : null;

  let fetched = 0;
  let pages = 0;
  let lastPageMs = 0;
  for (; ; page++) {
    if (!hasTimeForNextPage(run, lastPageMs)) {
      return { resource, mode: "delta", state: "partial", fetched, pages };
    }
    const t0 = run.now();
    let items: unknown[];
    try {
      ({ items } = await fetchPage(
        run,
        run.target.buildQuery({ updatedAtGteq: gteq, ...REFRESH_INCLUDE_OPTS }, page, run.perPage),
      ));
      maxSeen = maxString(maxSeen, maxUpdatedAt(items));
      const at = run.now();
      const guardOwner = run.owner;
      const statements = upsertStatements(db, resource, items, { generation, now: at, guardOwner });
      const isLast = items.length < run.perPage;
      statements.push(
        isLast
          ? db
              .prepare(
                `UPDATE sync_state SET last_synced_at = ?, last_sync_mode = 'delta', last_sync_status = 'success',
                 cursor_updated_at = COALESCE(?, cursor_updated_at), consecutive_failures = 0,
                 delta_next_page = NULL, delta_started_at = NULL, delta_max_updated_at = NULL,
                 query_version = ${CACHE_QUERY_VERSION}, delta_query_version = NULL
                 WHERE resource_name = ? AND refresh_owner = ?`,
              )
              .bind(isoSeconds(at), cursorAfterFull(maxSeen, startedAt), resource, guardOwner)
          : db
              .prepare(
                `UPDATE sync_state SET delta_next_page = ?, delta_started_at = ?, delta_max_updated_at = ?, delta_query_version = ?
                 WHERE resource_name = ? AND refresh_owner = ?`,
              )
              .bind(page + 1, startedAt, maxSeen, CACHE_QUERY_VERSION, resource, guardOwner),
      );
      const results = await db.batch(statements);
      assertOwned(run, results[results.length - 1]);
      fetched += items.length;
      pages++;
      if (isLast) return { resource, mode: "delta", state: "complete", fetched, pages };
    } catch (e) {
      if (e instanceof RefreshInProgressError) throw e; // ロックを失った: failed にせず止める。
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
  const run: Omit<Run, "owner"> & { owner?: string } = {
    deps,
    target,
    ctx: resolveContext(ctx, now),
    now,
    perPage: deps.perPage ?? DEFAULT_PER_PAGE,
    floorMs: deps.pageBudgetFloorMs ?? DEFAULT_PAGE_BUDGET_FLOOR_MS,
  };
  await ensureSchema(deps.db);
  return withLock(deps.db, target.resource, { now, owner: deps.owner }, async (lock) => {
    const owned: Run = { ...run, owner: lock.owner };
    const state = await getSyncState(deps.db, target.resource);
    // 仕様の一覧に updated_at_gteq が無いリソースは差分を取れない(送っても無視されて毎回全件になる)ので、delta は full として動かす。
    // 版が違う(旧コードのキャッシュ。アーカイブ・失注が欠けている)ときも、カーソルがないのと同じで全件が必要。
    // prune を見送った full のあと(must_full_resync=1)も同じ。取りこぼしが旧キャッシュにも無いので、delta では補えない(issue #40)。
    if (
      mode === "delta" &&
      state?.cursor_updated_at &&
      !mustSwitchToFull(state, now()) &&
      queryVersionCurrent(state.resource_name, state.query_version) &&
      supportsDelta(target.resource)
    ) {
      return runDelta(owned, state);
    }
    return runFull(owned, state);
  });
}

export { RefreshInProgressError };
