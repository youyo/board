import { beforeEach, describe, expect, it } from "vitest";
import {
  cacheFetchedEntity,
  countEntries,
  deleteAllEntries,
  deleteEntry,
  deleteOlderGenerations,
  getEntry,
  listEntries,
  upsertEntities,
} from "../../src/cache/resource-cache";
import { LOCK_STALE_MS } from "../../src/refresh/lock";
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

describe("cacheFetchedEntity: 新しい行を古い取得結果で上書きしない", () => {
  const T = Date.parse("2026-11-05T01:02:03Z"); // 取得を始めた時刻
  const fetchOpts = (over: { now?: number; fetchStartedAt?: number } = {}) => ({
    requestedId: 5,
    fetchStartedAt: over.fetchStartedAt ?? T,
    now: over.now ?? T + 5_000,
  });
  const nameOf = async (db: D1Database) =>
    ((await getEntry<{ name: string }>(db, "clients", 5))?.entity as { name: string } | undefined)
      ?.name;

  it("行がなければ書く", async () => {
    const db = testDb();
    expect(await cacheFetchedEntity(db, "clients", { id: 5, name: "new" }, fetchOpts())).toBe(true);
    expect(await nameOf(db)).toBe("new");
  });

  it("既存の行の fetched_at が fetchStartedAt より古ければ上書きする", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 5, name: "old" }], { now: T - 1_000 });
    expect(await cacheFetchedEntity(db, "clients", { id: 5, name: "new" }, fetchOpts())).toBe(true);
    expect(await nameOf(db)).toBe("new");
  });

  it("既存の行の fetched_at が fetchStartedAt 以降なら上書きしない(取得中に別の書き込みが先に新しい行を書いた)", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 5, name: "fresh" }], { now: T + 2_000 });
    expect(await cacheFetchedEntity(db, "clients", { id: 5, name: "stale" }, fetchOpts())).toBe(
      false,
    );
    expect(await nameOf(db)).toBe("fresh");
  });

  it("同じ秒は新しい行を守る側に倒す(上書きしない)", async () => {
    const db = testDb();
    await upsertEntities(db, "clients", [{ id: 5, name: "same-second" }], { now: T + 400 });
    expect(
      await cacheFetchedEntity(
        db,
        "clients",
        { id: 5, name: "stale" },
        fetchOpts({ fetchStartedAt: T + 900 }),
      ),
    ).toBe(false);
    expect(await nameOf(db)).toBe("same-second");
  });

  it("ロック中は書かない(既存の行があっても、なくても)", async () => {
    const db = testDb();
    await db
      .prepare(
        "INSERT INTO sync_state (resource_name, refresh_started_at, refresh_owner) VALUES ('clients', ?, 'other')",
      )
      .bind(new Date(T + 5_000 - LOCK_STALE_MS + 1_000).toISOString().replace(/\.\d{3}Z$/, "Z"))
      .run();
    expect(await cacheFetchedEntity(db, "clients", { id: 5, name: "x" }, fetchOpts())).toBe(false);
    expect(await getEntry(db, "clients", 5)).toBeNull();
    await upsertEntities(db, "clients", [{ id: 5, name: "old" }], { now: T - 1_000 });
    expect(await cacheFetchedEntity(db, "clients", { id: 5, name: "x" }, fetchOpts())).toBe(false);
    expect(await nameOf(db)).toBe("old");
  });

  it("世代は COALESCE(sync_state.full_generation, 0)。上書きでも書く時点の世代になる", async () => {
    const db = testDb();
    await cacheFetchedEntity(db, "clients", { id: 5, name: "a" }, fetchOpts());
    expect((await getEntry(db, "clients", 5))?.generation).toBe(0);
    await db
      .prepare("INSERT INTO sync_state (resource_name, full_generation) VALUES ('clients', 3)")
      .run();
    expect(
      await cacheFetchedEntity(
        db,
        "clients",
        { id: 5, name: "b" },
        fetchOpts({ fetchStartedAt: T + 10_000, now: T + 11_000 }),
      ),
    ).toBe(true);
    expect((await getEntry(db, "clients", 5))?.generation).toBe(3);
  });

  it("ID が要求と違えば書かない", async () => {
    const db = testDb();
    expect(await cacheFetchedEntity(db, "clients", { id: 6, name: "x" }, fetchOpts())).toBe(false);
    expect(await countEntries(db, "clients")).toBe(0);
  });
});
