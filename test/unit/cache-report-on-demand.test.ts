/** issue #38: 一覧も refresh も持たない書類の 4 リソースは、cache_status の refresh_state が on_demand。 */
import { beforeEach, describe, expect, it } from "vitest";
import { cacheReport } from "../../src/cache/admin";
import { resetCache, testDb } from "../helpers/d1";

beforeEach(async () => {
  await resetCache();
});

const DOCUMENTS = ["estimates", "orders", "deliveries", "receipts"];

describe("cacheReport: 書類リソースは on_demand", () => {
  it("未取得でも on_demand。row_count は単体取得で入った件数、cached_at は出さない", async () => {
    const db = testDb();
    const { resources } = await cacheReport(db);
    for (const name of DOCUMENTS) {
      expect(resources.find((r) => r.resource === name)).toEqual({
        resource: name,
        refresh_state: "on_demand",
        row_count: 0,
        full_prune_skipped: false,
        locked: false,
      });
    }
  });

  it("単体取得でキャッシュに入った件数が row_count になる(状態は on_demand のまま)", async () => {
    const db = testDb();
    await db.batch(
      [1, 2].map((id) =>
        db
          .prepare(
            "INSERT INTO resource_cache (resource_name, entity_id, payload_json, updated_at, fetched_at, generation) VALUES ('estimates', ?, '{}', NULL, '2026-11-04T00:00:00Z', 0)",
          )
          .bind(id),
      ),
    );
    const { resources } = await cacheReport(db);
    expect(resources.find((r) => r.resource === "estimates")).toMatchObject({
      refresh_state: "on_demand",
      row_count: 2,
    });
  });

  it("その他の 18 リソースは従来どおり(未取得は not_synced)で、on_demand にならない", async () => {
    const db = testDb();
    const { resources } = await cacheReport(db);
    const others = resources.filter((r) => !DOCUMENTS.includes(r.resource));
    expect(others).toHaveLength(18);
    for (const r of others) expect(r.refresh_state).toBe("not_synced");
  });
});
