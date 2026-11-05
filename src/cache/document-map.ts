/**
 * 書類 ID → 案件 ID の逆引き表と、その走査の進捗(M05。スキーマは v2)。
 *
 * 書類(見積・受注・納品・領収)の API は案件 ID を返さない。案件の一覧を response_group 付きで取ると書類 ID が入ってくるので、
 * 得た対応をここに残す。対応は不変(書類は作られた案件にずっと属する)なので、一度覚えれば二度と引き直さない。
 * 走査はページ単位で進捗を残す(締め切りで止まっても続きから再開できる)。ロックは refresh と同じ考え方の条件付き UPDATE。
 */

import { ensureSchema } from "./schema";
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

export function recordStatements(
  db: D1Database,
  kind: string,
  pairs: ReadonlyArray<{ documentId: number; projectId: number }>,
  nowMs: number,
): D1PreparedStatement[] {
  const at = isoSeconds(nowMs);
  return pairs.map((p) =>
    db
      .prepare(
        `INSERT OR REPLACE INTO document_project_map (document_kind, document_id, project_id, recorded_at)
         VALUES (?, ?, ?, ?)`,
      )
      .bind(kind, p.documentId, p.projectId, at),
  );
}

export async function getScanState(db: D1Database, kind: string): Promise<ScanState | null> {
  await ensureSchema(db);
  return db
    .prepare("SELECT * FROM document_scan_state WHERE document_kind = ?")
    .bind(kind)
    .first<ScanState>();
}

/** 進捗の更新(ロック列には触らない)。 */
export function saveProgressStatement(
  db: D1Database,
  kind: string,
  p: { nextPage: number | null; startedAt: string; completedAt: string | null },
): D1PreparedStatement {
  return db
    .prepare(
      `INSERT INTO document_scan_state (document_kind, next_page, started_at, completed_at)
       VALUES (?, ?, ?, ?)
       ON CONFLICT (document_kind) DO UPDATE SET
         next_page = excluded.next_page, started_at = excluded.started_at, completed_at = excluded.completed_at`,
    )
    .bind(kind, p.nextPage, p.startedAt, p.completedAt);
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
