/**
 * 書類 ID → 案件 ID の逆引き(Go 版 `find/reverse_map.go` の Workers 版)。
 *
 * 書類(見積・受注・納品・領収)のエンティティは案件 ID を持たない。Go は miss のたびに projects を
 * `response_group=<種別>` で全件走査して表を作っていた(実測 16〜32 秒、10 秒を超えたら ProjectID=0 に縮退)。
 * Workers は 1 回の呼び出しの締め切りが 30 秒なので、同じ方式では収まらない。そこでこうする:
 *
 * 1. 対応は D1 の表に残す(不変なので、一度覚えたら二度と引き直さない)。
 * 2. find 層が案件を response_group 付きで取るたびに、含まれる書類 ID を記録する(追加の通信なし)。
 * 3. 表にない書類は、ページ単位の走査で探す。1 ページ取るごとにその場で表と進捗(次のページ)を書き、
 *    見つけたら止める。**締め切りの手前(`SCAN_RESERVE_MS` を残して)で止め**、未解決として返す。
 *    次の呼び出しは保存した次のページから続けるので、数回の呼び出しで全件を覚える。
 * 4. 最後まで見て見つからなければ not_found。ただし走査のあとに作られた書類を拾うため、`COMPLETE_TTL_MS` の後の miss は走査し直す。
 *
 * 呼び出し側は『未解決』を ProjectID=0 + 警告で返す(秘密情報は含まない)。
 */

import type { BoardClient } from "../board/client";
import { type CallContext, resolveContext } from "../board/context";
import { isDeadlineError } from "../board/errors";
import { DEFAULT_PER_PAGE } from "../board/paging";
import { buildProjectsQuery, type ProjectEntity } from "../board/resources";
import {
  getProjectId,
  getScanState,
  recordStatements,
  releaseScanLock,
  type ScanState,
  saveProgressStatement,
  tryAcquireScanLock,
} from "../cache/document-map";
import { ensureSchema } from "../cache/schema";
import { type Clock, isoSeconds, systemClock } from "../cache/time";
import { BoardError } from "../errors";
import { DOCUMENT_KIND_INFO, type DocumentKind, isValidId } from "./document-kinds";

/** 走査をやめて残しておく時間。このあと案件・顧客の取得(補助情報)が待っている。 */
export const SCAN_RESERVE_MS = 8_000;
/** 1 ページ取るのに最低限必要な残り時間(直前のページの所要時間の 1.5 倍とのうち大きいほう)。 */
export const MIN_PAGE_BUDGET_MS = 1_500;
/** 最後まで走査した結果を信用する時間。この間の miss は再走査しない。 */
export const COMPLETE_TTL_MS = 10 * 60_000;
/** この時間を超えた走査ロックは stale とみなして奪える(走査は 1 回の呼び出し = 30 秒以内に終わる)。 */
export const SCAN_LOCK_STALE_MS = 2 * 60_000;
/**
 * 1 回の呼び出しで取るページ数の上限。締め切り(30 秒)に頼らず明示する。
 * 1 ページ = BOARD への通信 1 回 + D1 の batch 1 回。BOARD は 3 回/秒・3,000 回/日、Workers Free は 1 回の実行で通信 50 回までなので、
 * その範囲に余裕を持って収まる値にしている。上限に達したら締め切りのときと同じく、進捗を保存した状態で未解決(deadline)として終える。
 */
export const MAX_SCAN_PAGES_PER_CALL = 20;

export type IndexLookup =
  | { status: "hit"; projectId: number }
  /** 最後まで見たが見つからなかった。 */
  | { status: "miss"; reason: "not_found" }
  /** 走査が終わらなかった。次の呼び出しが続きから進む。 */
  | {
      status: "unresolved";
      reason: "deadline" | "scan_in_progress" | "api_error";
      /** api_error のときのエラーコード(メッセージは載せない)。 */
      errorCode?: string;
    };

export interface DocumentIndex {
  lookup(kind: DocumentKind, documentId: number, ctx?: CallContext): Promise<IndexLookup>;
  /** response_group=<kind> 付きで取った案件に含まれる書類 ID を覚える(通信なし)。 */
  record(kind: DocumentKind, projects: readonly ProjectEntity[]): Promise<void>;
}

export interface DocumentIndexDeps {
  db: D1Database;
  client: BoardClient;
  now?: Clock;
  perPage?: number;
}

function pairsOf(kind: DocumentKind, projects: readonly ProjectEntity[]) {
  const info = DOCUMENT_KIND_INFO[kind];
  // 案件 ID が不正な案件の書類は覚えない(上流の値を信用しない)。
  return projects.flatMap((p) =>
    isValidId(p.id) ? info.ids(p).map((documentId) => ({ documentId, projectId: p.id })) : [],
  );
}

function isCompletedFresh(state: ScanState | null, nowMs: number): boolean {
  const completedMs = state?.completed_at ? Date.parse(state.completed_at) : Number.NaN;
  return !Number.isNaN(completedMs) && nowMs - completedMs < COMPLETE_TTL_MS;
}

export function createDocumentIndex(deps: DocumentIndexDeps): DocumentIndex {
  const { db, client } = deps;
  const now = deps.now ?? systemClock;
  const perPage = deps.perPage ?? DEFAULT_PER_PAGE;

  async function record(kind: DocumentKind, projects: readonly ProjectEntity[]) {
    const pairs = pairsOf(kind, projects);
    if (pairs.length === 0) return;
    await ensureSchema(db);
    await db.batch(recordStatements(db, kind, pairs, now()));
  }

  async function lookup(
    kind: DocumentKind,
    documentId: number,
    callCtx?: CallContext,
  ): Promise<IndexLookup> {
    await ensureSchema(db);
    const known = await getProjectId(db, kind, documentId);
    if (known !== null) return { status: "hit", projectId: known };

    const start = now();
    const state0 = await getScanState(db, kind);
    // 先にロックなしで見る近道(完了して間もなければ、ロックも取らずに終わる)。ロックを取ったあとで読み直す。
    if (isCompletedFresh(state0, start)) return { status: "miss", reason: "not_found" };

    const ctx = resolveContext(callCtx, now);
    // 走査はここまでに使える時間(予約を残す)。1 ページ目に入る前の下限もここで見る。
    const budgetEnd = ctx.deadline - SCAN_RESERVE_MS;
    if (budgetEnd - start < MIN_PAGE_BUDGET_MS) return { status: "unresolved", reason: "deadline" };

    const owner = crypto.randomUUID();
    if (!(await tryAcquireScanLock(db, kind, owner, start, SCAN_LOCK_STALE_MS))) {
      return { status: "unresolved", reason: "scan_in_progress" };
    }
    try {
      // ロックを取ったので、状態を読み直す。読んでからロックを取るまでの間に、別の呼び出しが
      // 対応を覚えた・走査を完了した・進捗を進めたかもしれない(古い状態で再開すると、完了印を巻き戻す)。
      const known2 = await getProjectId(db, kind, documentId);
      if (known2 !== null) return { status: "hit", projectId: known2 };
      const state = await getScanState(db, kind);
      if (isCompletedFresh(state, now())) return { status: "miss", reason: "not_found" };
      // 完了済み(TTL 切れ)または未開始なら先頭から。途中なら保存した次のページから。
      const resume = state && !state.completed_at && state.next_page ? state.next_page : 1;
      let page = resume;
      const startedAt = resume === 1 ? isoSeconds(start) : (state?.started_at ?? isoSeconds(start));
      const group = DOCUMENT_KIND_INFO[kind].group;
      let lastPageMs = 0;
      let scanned = 0;
      for (;;) {
        if (scanned >= MAX_SCAN_PAGES_PER_CALL) return { status: "unresolved", reason: "deadline" };
        const remaining = budgetEnd - now();
        if (remaining < Math.max(MIN_PAGE_BUDGET_MS, lastPageMs * 1.5)) {
          return { status: "unresolved", reason: "deadline" };
        }
        const t0 = now();
        let rows: ProjectEntity[];
        try {
          // クライアントにも予約を残した締め切りを渡す(1 ページが長引いても予約の時間を食わない)。
          const res = await client.get(
            "/v1/projects",
            buildProjectsQuery({ responseGroup: group }, page, perPage),
            { signal: ctx.signal, deadline: budgetEnd },
          );
          if (!Array.isArray(res.body)) {
            return { status: "unresolved", reason: "api_error", errorCode: "api_error" };
          }
          rows = res.body as ProjectEntity[];
        } catch (e) {
          if (isDeadlineError(e)) return { status: "unresolved", reason: "deadline" };
          return {
            status: "unresolved",
            reason: "api_error",
            errorCode: e instanceof BoardError ? e.code : "generic_error",
          };
        }
        lastPageMs = now() - t0;
        scanned++;

        const done = rows.length < perPage;
        const pairs = pairsOf(kind, rows);
        // 1 ページ分の対応と進捗を同じ batch で書く(途中で止まっても整合する)。
        // すべて走査ロックの所有者を条件にする。奪われていれば何も書かれず、最後の文の changes が 1 にならない。
        const results = await db.batch([
          ...recordStatements(db, kind, pairs, now(), owner),
          saveProgressStatement(
            db,
            kind,
            {
              nextPage: done ? null : page + 1,
              startedAt,
              completedAt: done ? isoSeconds(now()) : null,
            },
            owner,
          ),
        ]);
        if (results[results.length - 1]?.meta.changes !== 1) {
          // ロックを失った(stale として奪われた)。以後の書き込みも走査も中止する。
          return { status: "unresolved", reason: "scan_in_progress" };
        }
        const found = pairs.find((p) => p.documentId === documentId);
        if (found) return { status: "hit", projectId: found.projectId };
        if (done) return { status: "miss", reason: "not_found" };
        page++;
      }
    } finally {
      await releaseScanLock(db, kind, owner);
    }
  }

  return { lookup, record };
}
