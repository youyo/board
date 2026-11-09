/**
 * 書類 ID → 案件 ID の逆引き表と、その走査の進捗(M05。スキーマは v2)。
 *
 * 書類(見積・受注・納品・領収)の API は案件 ID を返さない。案件の一覧を response_group 付きで取ると書類 ID が入ってくるので、
 * 得た対応をここに残す。対応は不変(書類は作られた案件にずっと属する)なので、一度覚えれば二度と引き直さない。
 * 走査はページ単位で進捗を残す(締め切りで止まっても続きから再開できる)。ロックは refresh と同じ考え方の条件付き UPDATE。
 */

import { ensureSchema } from "./schema";
import { CACHE_QUERY_VERSION } from "./sync-state";
import { isoSeconds } from "./time";

export interface ScanState {
  document_kind: string;
  /** 次に取るページ。NULL なら未開始、または完了。 */
  next_page: number | null;
  started_at: string | null;
  /** 最後のページまで取り終えた時刻。 */
  completed_at: string | null;
  lock_started_at: string | null;
  lock_owner: string | null;
  /** 進捗・完了を保存した版(v4)。CACHE_QUERY_VERSION と一致するときだけ有効。NULL は旧コードの走査。 */
  query_version: number | null;
}

/** 現在の版で保存した走査状態か(旧コードの進捗・完了は、アーカイブ・失注の案件を飛ばしているので使わない)。 */
export function scanVersionCurrent(scan: ScanState | null): boolean {
  return scan?.query_version === CACHE_QUERY_VERSION;
}

export async function getProjectId(
  db: D1Database,
  kind: string,
  documentId: number,
): Promise<number | null> {
  await ensureSchema(db);
  const row = await db
    .prepare(
      "SELECT project_id FROM document_project_map WHERE document_kind = ? AND document_id = ?",
    )
    .bind(kind, documentId)
    .first<{ project_id: number }>();
  return row?.project_id ?? null;
}

/**
 * 1 文に入れる行数。D1 は 1 文あたりのバインド変数が 100 まで(Workers の D1 の制限。docs: Limits の
 * "Maximum bound parameters per query")。1 行 4 個 + 所有者の条件 2 個で 24 行 = 98 個に収める。
 */
export const MAP_ROWS_PER_STATEMENT = 24;

const MAP_COLUMNS = "(document_kind, document_id, project_id, recorded_at)";
const ROW_PLACEHOLDER = "(?, ?, ?, ?)";

/**
 * 対応の記録。複数行の 1 文にまとめて、D1 に流すステートメント数を減らす(1 行 1 文だと 1 ページで数百文になる)。
 * `guardOwner` を渡すと、走査ロックの所有者がその値のときだけ書く(同じ batch の中で判定する。stale を奪われた元の保持者の書き込みを無効にする)。
 */
export function recordStatements(
  db: D1Database,
  kind: string,
  pairs: ReadonlyArray<{ documentId: number; projectId: number }>,
  nowMs: number,
  guardOwner?: string,
): D1PreparedStatement[] {
  const at = isoSeconds(nowMs);
  const out: D1PreparedStatement[] = [];
  for (let i = 0; i < pairs.length; i += MAP_ROWS_PER_STATEMENT) {
    const chunk = pairs.slice(i, i + MAP_ROWS_PER_STATEMENT);
    const values = chunk.map(() => ROW_PLACEHOLDER).join(", ");
    const params = chunk.flatMap((p) => [kind, p.documentId, p.projectId, at]);
    if (guardOwner === undefined) {
      out.push(
        db
          .prepare(`INSERT OR REPLACE INTO document_project_map ${MAP_COLUMNS} VALUES ${values}`)
          .bind(...params),
      );
    } else {
      out.push(
        db
          .prepare(
            `INSERT OR REPLACE INTO document_project_map ${MAP_COLUMNS}
             SELECT * FROM (VALUES ${values})
             WHERE EXISTS (SELECT 1 FROM document_scan_state WHERE document_kind = ? AND lock_owner = ?)`,
          )
          .bind(...params, kind, guardOwner),
      );
    }
  }
  return out;
}

export async function getScanState(db: D1Database, kind: string): Promise<ScanState | null> {
  await ensureSchema(db);
  return db
    .prepare("SELECT * FROM document_scan_state WHERE document_kind = ?")
    .bind(kind)
    .first<ScanState>();
}

/**
 * 進捗の更新(ロック列には触らない)。走査ロックの所有者が `owner` のときだけ書く。
 * 結果の `meta.changes` が 1 でなければロックを失っている(呼び出し側は以後の書き込みを中止する)。
 * 行は tryAcquireScanLock が作るので、UPDATE で足りる。
 */
export function saveProgressStatement(
  db: D1Database,
  kind: string,
  p: { nextPage: number | null; startedAt: string; completedAt: string | null },
  owner: string,
): D1PreparedStatement {
  return db
    .prepare(
      `UPDATE document_scan_state SET next_page = ?, started_at = ?, completed_at = ?, query_version = ?
       WHERE document_kind = ? AND lock_owner = ?`,
    )
    .bind(p.nextPage, p.startedAt, p.completedAt, CACHE_QUERY_VERSION, kind, owner);
}

/** 取れたら true。待たない。 */
export async function tryAcquireScanLock(
  db: D1Database,
  kind: string,
  owner: string,
  nowMs: number,
  staleMs: number,
): Promise<boolean> {
  await ensureSchema(db);
  await db
    .prepare("INSERT OR IGNORE INTO document_scan_state (document_kind) VALUES (?)")
    .bind(kind)
    .run();
  const r = await db
    .prepare(
      `UPDATE document_scan_state SET lock_started_at = ?, lock_owner = ?
       WHERE document_kind = ? AND (lock_started_at IS NULL OR lock_started_at < ?)`,
    )
    .bind(isoSeconds(nowMs), owner, kind, isoSeconds(nowMs - staleMs))
    .run();
  return r.meta.changes === 1;
}

/** 自分のロックだけを解放する。 */
export async function releaseScanLock(db: D1Database, kind: string, owner: string): Promise<void> {
  await db
    .prepare(
      `UPDATE document_scan_state SET lock_started_at = NULL, lock_owner = NULL
       WHERE document_kind = ? AND lock_owner = ?`,
    )
    .bind(kind, owner)
    .run();
}
