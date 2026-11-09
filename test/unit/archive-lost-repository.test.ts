/**
 * issue #35: API 経路の既定補完(未指定ならアーカイブ・失注も含める。BOARD の既定とは逆)、逆引き走査、find 層の名前解決。
 * BOARD のモックは include フラグの有無で返す件数を変える(フラグが無いと、アーカイブ済み・失注の案件が出てこない)。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LIST_FILTER_SUPPORT } from "../../src/board/openapi-support";
import { createRepositories } from "../../src/repository";
import { createDocumentIndex } from "../../src/repository/document-index";
import { SPECS } from "../../src/repository/resources";
import { callApiTool } from "../helpers/api-tool";
import { BASE, pagedHandler, testClient } from "../helpers/board-list";
import { fakeClock, resetCache, testDb } from "../helpers/d1";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const ARCHIVE_RESOURCES = Object.entries(LIST_FILTER_SUPPORT)
  .filter(([, s]) => s.includeArchiveFlg)
  .map(([name]) => name);
const NO_ARCHIVE_RESOURCES = Object.entries(LIST_FILTER_SUPPORT)
  .filter(([, s]) => !s.includeArchiveFlg)
  .map(([name]) => name);

const pathOf = (r: string) => (SPECS[r as keyof typeof SPECS] as { path: string }).path;

describe("viaApi の既定補完: include_archive_flg / include_lost_flg を未指定なら true にする", () => {
  // perPage はキャッシュで絞れない項目なので、どのリソースでも API 経路になる(中身は件数に影響しない)。
  async function listVia(resource: string, filter: Record<string, unknown>) {
    const repos = createRepositories({ db: testDb(), client: testClient() }) as unknown as Record<
      string,
      { list: (f: object) => Promise<unknown> }
    >;
    const api = mockOutboundFetch({
      [`GET ${BASE}${pathOf(resource)}`]: pagedHandler(() => []),
    });
    await repos[resource]?.list({ perPage: 50, ...filter });
    return new URL(api.calls[0]?.url ?? "").searchParams;
  }

  it.each(ARCHIVE_RESOURCES)(
    "%s: 未指定なら include_archive_flg=1(projects は include_lost_flg=1 も)",
    async (resource) => {
      const q = await listVia(resource, {});
      expect(q.get("include_archive_flg")).toBe("1");
      expect(q.get("include_lost_flg")).toBe(resource === "projects" ? "1" : null);
    },
  );

  it.each(NO_ARCHIVE_RESOURCES)("%s: 非対応なので何も足さない", async (resource) => {
    const q = await listVia(resource, {});
    expect(q.get("include_archive_flg")).toBeNull();
    expect(q.get("include_lost_flg")).toBeNull();
  });

  it("明示の false は =0 のまま送る(projects の include_lost_flg も)", async () => {
    const q = await listVia("projects", { includeArchiveFlg: false, includeLostFlg: false });
    expect(q.get("include_archive_flg")).toBe("0");
    expect(q.get("include_lost_flg")).toBe("0");
  });

  it("明示の true は =1", async () => {
    const q = await listVia("projects", { includeArchiveFlg: true, includeLostFlg: true });
    expect(q.get("include_archive_flg")).toBe("1");
    expect(q.get("include_lost_flg")).toBe("1");
  });

  it("片方だけ指定: 指定していない方は補完する(include_archive_flg=false でも include_lost_flg は =1)", async () => {
    const q = await listVia("projects", { includeArchiveFlg: false });
    expect(q.get("include_archive_flg")).toBe("0");
    expect(q.get("include_lost_flg")).toBe("1");
    const q2 = await listVia("projects", { includeLostFlg: false });
    expect(q2.get("include_archive_flg")).toBe("1");
    expect(q2.get("include_lost_flg")).toBe("0");
  });

  it("明示の false は『指定あり・キー外』なので、同期済みでもキャッシュではなく API に回る", async () => {
    const repos = createRepositories({ db: testDb(), client: testClient() });
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/clients`]: pagedHandler(() => [{ id: 1, name: "A" }]),
    });
    await repos.clients.refresh("full");
    const before = api.calls.length;
    const r = await repos.clients.list({ includeArchiveFlg: false });
    expect(r.source).toBe("api");
    expect(api.calls.length).toBeGreaterThan(before);
    expect(new URL(api.calls.at(-1)?.url ?? "").searchParams.get("include_archive_flg")).toBe("0");
  });

  it("ツール経由(list_projects に include_archive_flg を渡さない)でも =1 / include_lost_flg=1", async () => {
    const r = await callApiTool(
      "list_projects",
      { name_cont: "x" },
      { [`GET ${BASE}/v1/projects`]: () => jsonResponse([]) },
    );
    const q = new URL(r.calls[0]?.url ?? "").searchParams;
    expect(q.get("include_archive_flg")).toBe("1");
    expect(q.get("include_lost_flg")).toBe("1");
  });
});

describe("書類 → 案件の逆引き走査は、アーカイブ・失注も含めて全ページ走査する", () => {
  it("全ページに include_archive_flg=1 と include_lost_flg=1 が付く", async () => {
    const clock = fakeClock();
    const index = createDocumentIndex({
      db: testDb(),
      client: testClient(clock.now),
      now: clock.now,
      perPage: 2,
    });
    const projects = Array.from({ length: 5 }, (_, i) => ({
      id: i + 1,
      name: `案件${i + 1}`,
      estimate: { id: 1000 + i + 1 },
    }));
    const api = mockOutboundFetch({ [`GET ${BASE}/v1/projects`]: pagedHandler(() => projects) });
    const r = await index.lookup("estimate", 99999); // 見つからない = 最後まで走査する
    expect(r.status).toBe("miss");
    expect(api.calls.length).toBeGreaterThanOrEqual(3);
    for (const c of api.calls) {
      const q = new URL(c.url).searchParams;
      expect(q.get("include_archive_flg"), c.url).toBe("1");
      expect(q.get("include_lost_flg"), c.url).toBe("1");
    }
  });
});

// --- find 層: アーカイブ済み・失注の同名案件 ---------------------------------------------------

const ACTIVE = { id: 1, name: "保守案件", project_no: 1001, order_status: 1 };
const ARCHIVED = { id: 2, name: "保守案件", project_no: 1002, order_status: 9 };

/** include_archive_flg=1 かつ include_lost_flg=1 のときだけアーカイブ済み・失注の案件を返す(実 API の既定は除外)。 */
function projectsHandler(
  extra: (p: Record<string, unknown>) => Record<string, unknown> = (p) => p,
) {
  return pagedHandler(
    () => [],
    (_page, q) => {
      const includeAll = q.get("include_archive_flg") === "1" && q.get("include_lost_flg") === "1";
      const rows = (includeAll ? [ACTIVE, ARCHIVED] : [ACTIVE]).map(extra);
      return jsonResponse(rows);
    },
  );
}

describe("find 層: アーカイブ済み・失注の同名案件が名前解決に出る", () => {
  const PROJECTS = `GET ${BASE}/v1/projects`;

  it("find_invoices project_name(キャッシュ未同期 = API 経路): 曖昧エラーで候補に 2 件", async () => {
    const r = await callApiTool(
      "find_invoices",
      { project_name: "保守案件" },
      { [PROJECTS]: projectsHandler() },
    );
    expect(r.isError, r.text).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain("2 hits");
    expect(r.json.error.message).toContain("id=1");
    expect(r.json.error.message).toContain("id=2");
  });

  it("find_invoices project_name(キャッシュ同期済み): 曖昧エラーで候補に 2 件", async () => {
    const synced = await callApiTool(
      "list_projects",
      { refresh: "full", limit: 1 },
      { [PROJECTS]: projectsHandler() },
    );
    expect(synced.isError, synced.text).toBe(false);
    vi.restoreAllMocks();

    const r = await callApiTool("find_invoices", { project_name: "保守案件" }, {});
    expect(r.isError, r.text).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain("2 hits");
    expect(r.json.error.message).toContain("id=2");
    expect(r.calls).toHaveLength(0); // キャッシュだけで解決した
  });

  it("find_projects name(API 経路): アーカイブ済み・失注も出る", async () => {
    const r = await callApiTool(
      "find_projects",
      { name: "保守案件" },
      { [PROJECTS]: projectsHandler() },
    );
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items.map((x: { Project: { id: number } }) => x.Project.id)).toEqual([1, 2]);
  });

  it("find_projects name(キャッシュ同期済み): アーカイブ済み・失注も出る", async () => {
    await callApiTool(
      "list_projects",
      { refresh: "full", limit: 1 },
      { [PROJECTS]: projectsHandler() },
    );
    vi.restoreAllMocks();
    const r = await callApiTool("find_projects", { name: "保守案件" }, {});
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items.map((x: { Project: { id: number } }) => x.Project.id)).toEqual([1, 2]);
  });

  it("find_estimates project_name(response_group つきの API 経路): アーカイブ済み・失注の案件の見積も出る", async () => {
    const r = await callApiTool(
      "find_estimates",
      { project_name: "保守案件" },
      {
        [PROJECTS]: projectsHandler((p) => ({ ...p, estimate: { id: 100 + Number(p.id) } })),
        [`GET ${BASE}/v1/documents/estimates/*`]: (call) => {
          const id = Number(new URL(call.url).pathname.split("/").pop());
          return jsonResponse({ id, estimate_no: `E${id}` });
        },
      },
    );
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items.map((x: { Estimate: { id: number } }) => x.Estimate.id)).toEqual([
      101, 102,
    ]);
    const projectCall = r.calls.find((c) => c.url.includes("/v1/projects"));
    expect(new URL(projectCall?.url ?? "").searchParams.get("include_archive_flg")).toBe("1");
    expect(new URL(projectCall?.url ?? "").searchParams.get("response_group")).toBe("estimate");
  });
});
