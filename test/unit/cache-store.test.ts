import { beforeEach, describe, expect, it } from "vitest";
import {
  countEntries,
  deleteAllEntries,
  deleteEntry,
  deleteOlderGenerations,
  getEntry,
  listEntries,
  upsertEntities,
} from "../../src/cache/resource-cache";
import { resetCache, testDb } from "../helpers/d1";

const NOW = Date.parse("2026-11-05T01:02:03Z");

beforeEach(async () => {
  await resetCache();
});

describe("resource_cache: upsert / 取得 / 削除", () => {
  it("エンティティの JSON を壊さずに往復する(文字列の金額・日本語・未知のフィールド)", async () => {
    const db = testDb();
    const entity = {
      id: 7,
      name: "株式会社テスト",
      total: "90000.0",
      updated_at: "2024-12-26T10:14:11.000+09:00",
      future_field: { nested: [1, 2, { a: null }] },
    };
    await upsertEntities(db, "clients", [entity], { now: NOW });
    const got = await getEntry<typeof entity>(db, "clients", 7);
    expect(got?.entity).toEqual(entity);
    expect(got?.updated_at).toBe("2024-12-26T10:14:11.000+09:00");
    expect(got?.fetched_at).toBe("2026-11-05T01:02:03Z");
  });

  it("同じ id は置き換わり、世代も新しくなる", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 1, name: "old" }], { now: NOW, generation: 1 });
    await upsertEntities(db, "clients", [{ id: 1, name: "new" }], { now: NOW, generation: 2 });
    expect((await getEntry<{ name: string }>(db, "clients", 1))?.entity.name).toBe("new");
    expect(await countEntries(db, "clients")).toBe(1);
    const row = await db
      .prepare(
        "SELECT generation FROM resource_cache WHERE resource_name='clients' AND entity_id=1",
      )
      .first<{ generation: number }>();
    expect(row?.generation).toBe(2);
  });

  it("存在しない id は null", async () => {
    expect(await getEntry(testDb(), "clients", 999)).toBeNull();
  });

  it("リソースごとに分かれる(同じ id でも別の行)", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 1, name: "c" }], { now: NOW });
    await upsertEntities(db, "vendors", [{ id: 1, name: "v" }], { now: NOW });
    expect((await getEntry<{ name: string }>(db, "clients", 1))?.entity.name).toBe("c");
    expect((await getEntry<{ name: string }>(db, "vendors", 1))?.entity.name).toBe("v");
  });

  it("id のないエンティティは api_error(キャッシュを壊さない)", async () => {
    const db = testDb();
    await expect(
      upsertEntities(db, "clients", [{ id: 1 }, { name: "no id" }], { now: NOW }),
    ).rejects.toMatchObject({
      code: "api_error",
    });
    // バッチは原子的: 1 件目も入っていない。
    expect(await countEntries(db, "clients")).toBe(0);
  });

  it("空配列は何もしない", async () => {
    expect(await upsertEntities(testDb(), "clients", [], { now: NOW })).toBe(0);
  });

  it("250 件を 1 回で入れられる", async () => {
    const db = testDb();
    const rows = Array.from({ length: 250 }, (_, i) => ({ id: i + 1, name: `n${i}` }));
    expect(await upsertEntities(db, "clients", rows, { now: NOW })).toBe(250);
    expect(await countEntries(db, "clients")).toBe(250);
  });

  it("listEntries は updated_at の昇順(NULL が先頭)、同じなら id 順", async () => {
    const db = testDb();
    await upsertEntities(
      db,
      "clients",
      [
        { id: 3, updated_at: "2026-01-02T00:00:00Z" },
        { id: 1, updated_at: "2026-01-03T00:00:00Z" },
        { id: 4 },
        { id: 2, updated_at: "2026-01-02T00:00:00Z" },
      ],
      { now: NOW },
    );
    const list = await listEntries<{ id: number }>(db, "clients");
    expect(list.map((e) => e.entity.id)).toEqual([4, 2, 3, 1]);
  });

  it("空のリソースの list は空配列", async () => {
    expect(await listEntries(testDb(), "clients")).toEqual([]);
  });

  it("deleteEntry は 1 件、deleteAllEntries はそのリソースだけを消す", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 1 }, { id: 2 }], { now: NOW });
    await upsertEntities(db, "vendors", [{ id: 1 }], { now: NOW });
    await deleteEntry(db, "clients", 1);
    expect(await countEntries(db, "clients")).toBe(1);
    await deleteEntry(db, "clients", 12345); // 無くてもエラーにならない
    await deleteAllEntries(db, "clients");
    expect(await countEntries(db, "clients")).toBe(0);
    expect(await countEntries(db, "vendors")).toBe(1);
  });

  it("deleteOlderGenerations は指定の世代より古い行だけを消す", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 1 }, { id: 2 }], { now: NOW, generation: 1 });
    await upsertEntities(db, "clients", [{ id: 3 }], { now: NOW, generation: 2 });
    await upsertEntities(db, "vendors", [{ id: 9 }], { now: NOW, generation: 1 });
    await deleteOlderGenerations(db, "clients", 2);
    expect((await listEntries<{ id: number }>(db, "clients")).map((e) => e.entity.id)).toEqual([3]);
    expect(await countEntries(db, "vendors")).toBe(1);
  });
});
