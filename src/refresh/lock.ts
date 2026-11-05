/**
 * refresh の排他ロック(Go 版 `internal/refresh/lock.go` の TryLock に相当)。
 *
 * Go はプロセス内 mutex + sync_state の `refresh_started_at` だったが、Workers は isolate が複数あるので
 * D1 の条件付き UPDATE だけで取る(読んでから書く方式にしない)。取得の成否は `meta.changes` で判定する。
 * 待たない: 取れなければ即 RefreshInProgressError(retry_after_seconds 30)。
 */

import { ensureSchema } from "../cache/schema";
import { ensureRowStatement, getSyncState } from "../cache/sync-state";
import { type Clock, isoSeconds, systemClock } from "../cache/time";
import { REFRESH_RETRY_AFTER_SECONDS, RefreshInProgressError } from "../errors";

export { REFRESH_RETRY_AFTER_SECONDS };

/** この時間を超えたロックは stale とみなして奪取できる(Go の staleLockTimeout)。 */
export const LOCK_STALE_MS = 10 * 60_000;

export interface LockOptions {
  now?: Clock;
  /** 取得者の識別子。既定は呼び出しごとのランダムな UUID。 */
  owner?: string;
}

export interface Lock {
  resource: string;
  owner: string;
}

export async function tryAcquireLock(
  db: D1Database,
  resource: string,
  opts: LockOptions = {},
): Promise<Lock> {
  await ensureSchema(db);
  const now = (opts.now ?? systemClock)();
  const owner = opts.owner ?? crypto.randomUUID();
  await ensureRowStatement(db, resource).run();
  const r = await db
    .prepare(
      `UPDATE sync_state SET refresh_started_at = ?, refresh_owner = ?
       WHERE resource_name = ? AND (refresh_started_at IS NULL OR refresh_started_at < ?)`,
    )
    .bind(isoSeconds(now), owner, resource, isoSeconds(now - LOCK_STALE_MS))
    .run();
  if (r.meta.changes === 1) return { resource, owner };

  // 取れなかった: 保持者と経過時間を best-effort で添える。
  const st = await getSyncState(db, resource);
  const started = st?.refresh_started_at ? Date.parse(st.refresh_started_at) : Number.NaN;
  throw new RefreshInProgressError(resource, {
    holder: st?.refresh_owner ?? "",
    elapsedSeconds: Number.isNaN(started) ? 0 : Math.max(0, Math.floor((now - started) / 1000)),
  });
}

/** 自分のロックだけを解放する(stale を奪った別の取得者のロックは解放しない)。 */
export async function releaseLock(db: D1Database, resource: string, owner: string): Promise<void> {
  await db
    .prepare(
      `UPDATE sync_state SET refresh_started_at = NULL, refresh_owner = NULL
       WHERE resource_name = ? AND refresh_owner = ?`,
    )
    .bind(resource, owner)
    .run();
}

/** 取得 → fn → 解放。成功・失敗・例外のどれでも finally で解放する。取れなければ fn は呼ばない。 */
export async function withLock<T>(
  db: D1Database,
  resource: string,
  opts: LockOptions,
  fn: (lock: Lock) => Promise<T>,
): Promise<T> {
  const lock = await tryAcquireLock(db, resource, opts);
  try {
    return await fn(lock);
  } finally {
    await releaseLock(db, resource, lock.owner);
  }
}
