import { beforeEach, describe, expect, it } from "vitest";
import { recordStatements, tryAcquireScanLock } from "../../src/cache/document-map";
import { upsertEntities } from "../../src/cache/resource-cache";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { tryAcquireLock } from "../../src/refresh/lock";
import { callApiTool } from "../helpers/api-tool";
import { resetCache, testDb } from "../helpers/d1";

const NOW = Date.now();

async function count(table: string, where = "1=1"): Promise<number> {
  const r = await testDb()
    .prepare(`SELECT count(*) AS c FROM ${table} WHERE ${where}`)
    .first<{ c: number }>();
  return r?.c ?? 0;
}

async function seed() {
  const db = testDb();
  await upsertEntities(db, "clients", [{ id: 1 }, { id: 2 }], { now: NOW });
  await upsertEntities(db, "vendors", [{ id: 1 }], { now: NOW });
  await db.batch(recordStatements(db, "estimate", [{ documentId: 10, projectId: 1 }], NOW));
}

beforeEach(async () => {
  await resetCache();
});

describe("ツール定義", () => {
  const byName = new Map(ALL_TOOLS.map((t) => [t.name, t]));

  it("cache_status は読み取り専用、cache_clear は destructiveHint つき", () => {
    expect(byName.get("cache_status")?.annotations).toMatchObject({ readOnlyHint: true });
    expect(byName.get("cache_clear")?.annotations).toMatchObject({
      readOnlyHint: false,
      destructiveHint: true,
    });
  });

  it("cache_expire は作らない(読み取りに影響しないため)", () => {
    expect(byName.has("cache_expire")).toBe(false);
  });

  it("description に『いつ使うか』が書かれている", () => {
    expect(byName.get("cache_status")?.description).toMatch(/refresh_state|partial/);
    expect(byName.get("cache_clear")?.description).toMatch(/destructive|delete/i);
  });
});

describe("cache_status", () => {
  it("全 22 リソースと、逆引きの走査状態 4 種を返す", async () => {
    await seed();
    const r = await callApiTool("cache_status", {});
    expect(r.isError).toBe(false);
    expect(r.json.resources).toHaveLength(22);
    expect(
      r.json.resources.find((x: { resource: string }) => x.resource === "clients"),
    ).toMatchObject({
      row_count: 2,
      refresh_state: "not_synced",
      locked: false,
      full_prune_skipped: false,
    });
    expect(r.json.document_scans).toHaveLength(4);
    expect(r.json.document_scans[0]).toMatchObject({ document_kind: "estimate", mapped_rows: 1 });
    expect(r.calls).toEqual([]);
  });

  it("BOARD API を呼ばず、状態も変えない", async () => {
    await seed();
    await callApiTool("cache_status", {});
    expect(await count("resource_cache")).toBe(3);
    expect(await count("sync_state")).toBe(0);
  });
});

describe("cache_clear", () => {
  it("resources を指定するとそのリソースだけ消す。逆引きの対応表は消さない", async () => {
    await seed();
    const r = await callApiTool("cache_clear", { resources: ["clients"] });
    expect(r.isError).toBe(false);
    expect(r.json).toEqual({ cleared: ["clients"], document_map_cleared: false });
    expect(await count("resource_cache", "resource_name = 'clients'")).toBe(0);
    expect(await count("resource_cache", "resource_name = 'vendors'")).toBe(1);
    expect(await count("document_project_map")).toBe(1);
  });

  it("resources を省略すると全リソース(逆引きは残る)", async () => {
    await seed();
    const r = await callApiTool("cache_clear", {});
    expect(r.json.cleared).toHaveLength(22);
    expect(await count("resource_cache")).toBe(0);
    expect(await count("document_project_map")).toBe(1);
  });

  it("include_document_map: true で逆引きの対応表と走査状態も消す", async () => {
    await seed();
    const r = await callApiTool("cache_clear", {
      resources: ["vendors"],
      include_document_map: true,
    });
    expect(r.json).toEqual({ cleared: ["vendors"], document_map_cleared: true });
    expect(await count("document_project_map")).toBe(0);
    expect(await count("resource_cache", "resource_name = 'clients'")).toBe(2);
  });

  it("refresh 中のリソースがあれば refresh_in_progress(何も消さない)", async () => {
    await seed();
    await tryAcquireLock(testDb(), "clients", { owner: "A" });
    const r = await callApiTool("cache_clear", {});
    expect(r.isError).toBe(true);
    expect(r.json.error).toMatchObject({
      code: "refresh_in_progress",
      retryable: true,
      retry_after_seconds: 30,
    });
    expect(await count("resource_cache")).toBe(3);
  });

  it("逆引きの走査中に include_document_map を指定すると、リソースは消えたうえで document_map_cleared: false と理由を返す", async () => {
    await seed();
    await tryAcquireScanLock(testDb(), "order", "other", Date.now(), 120_000);
    const r = await callApiTool("cache_clear", {
      resources: ["clients"],
      include_document_map: true,
    });
    expect(r.isError).toBe(false);
    expect(r.json).toMatchObject({
      cleared: ["clients"],
      document_map_cleared: false,
      document_map_error: { code: "refresh_in_progress", retry_after_seconds: 30 },
    });
    expect(await count("document_project_map")).toBe(1);
  });

  it("未知のリソース名は argument_error", async () => {
    const r = await callApiTool("cache_clear", { resources: ["nope"] });
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
  });

  it("空配列の resources は argument_error で、何も消さない(全件削除と取り違えない)", async () => {
    const r = await callApiTool("cache_clear", { resources: [] });
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
  });

  it("未知の引数は argument_error", async () => {
    const r = await callApiTool("cache_clear", { everything: true });
    expect(r.json.error.code).toBe("argument_error");
  });
});
