import { env } from "cloudflare:workers";
import { ensureSchema } from "../../src/cache/schema";

/** miniflare の D1(wrangler.jsonc の DB 束縛)。D1 の状態はテストファイルごとに隔離され、同じファイルの中では共有される。 */
export const testDb = (): D1Database => env.DB;

/** スキーマを適用して全テーブルを空にする(DROP はしない。ensureSchema の記憶と食い違うため)。各テストの beforeEach で呼ぶ。 */
export async function resetCache(db: D1Database = testDb()): Promise<D1Database> {
  await ensureSchema(db);
  await db.batch([
    db.prepare("DELETE FROM resource_cache"),
    db.prepare("DELETE FROM sync_state"),
    db.prepare("DELETE FROM cache_meta WHERE key <> 'db_schema_version'"),
  ]);
  return db;
}

/** 進められる時計(epoch ms)。 */
export function fakeClock(start = Date.parse("2026-11-05T00:00:00Z")) {
  let t = start;
  return {
    now: () => t,
    advance(ms: number) {
      t += ms;
    },
    set(ms: number) {
      t = ms;
    },
  };
}
