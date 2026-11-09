import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createBoardClient } from "../../src/board/client";
import { getSyncState } from "../../src/cache/sync-state";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { tryAcquireLock } from "../../src/refresh/lock";
import { createRepositories } from "../../src/repository";
import { BASE, callApiTool, describeApiCall } from "../helpers/api-tool";
import { makeEntities, pagedHandler } from "../helpers/board-list";
import { resetCache, testDb } from "../helpers/d1";
import { userEntity } from "../helpers/entities";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";

afterEach(() => vi.restoreAllMocks());
beforeEach(async () => {
  await resetCache();
});

const ok = (rows: unknown[] = []) => pagedHandler(() => rows);

/**
 * 引数 → BOARD への問い合わせ(Go 版 CLI のフラグ → クエリ)の突き合わせ。
 * 期待値は Go の `internal/cli/api_*.go` のフラグと `internal/boardapi/*.go` のクエリ名から作った。
 * 非ゼロのフィルタなので、キャッシュが空でも API を直接呼ぶ(nonzero-api、または cache-first の API フォールバック)。
 */
const QUERY_CASES: { tool: string; args: Record<string, unknown>; path: string; call: string }[] = [
  {
    tool: "list_clients",
    args: {
      name_cont: "株式",
      name_disp_cont: "略",
      invoice_system_number_eq: "T123",
      custom_no_eq: "C-1",
      tags: ["A", "B"],
      response_group: "large",
      updated_at_gteq: "2026-01-01 00:00:00",
      updated_at_lteq: "2026-02-01 00:00:00",
      include_archive_flg: true,
    },
    path: "/v1/clients",
    call:
      "GET /v1/clients?custom_no_eq=C-1&include_archive_flg=1&invoice_system_number_eq=T123" +
      "&name_cont=株式&name_disp_cont=略&response_group=large&tags[]=A&tags[]=B" +
      "&updated_at_gteq=2026-01-01 00:00:00&updated_at_lteq=2026-02-01 00:00:00",
  },
  {
    tool: "list_projects",
    args: {
      name_cont: "案件",
      client_id: 7,
      client_name_cont: "顧客",
      order_status_in: [1, 4],
      delivery_status_in: [2],
      project_no_eq: "P-1",
      management_no_eq: "M-1",
      delivery_date_gteq: "2026-03-01",
      delivery_date_lteq: "2026-03-31",
      invoice_date_gteq: "2026-04-01",
      invoice_date_lteq: "2026-04-30",
      invoice_timing_kbn_in: [1, 2],
      tags: ["x"],
      response_group: "all",
      created_at_gteq: "2026-01-01 00:00:00",
      created_at_lteq: "2026-01-31 00:00:00",
      include_archive_flg: false,
      include_lost_flg: true,
    },
    path: "/v1/projects",
    call:
      "GET /v1/projects?client_id_eq=7&client_name_cont=顧客&created_at_gteq=2026-01-01 00:00:00" +
      "&created_at_lteq=2026-01-31 00:00:00&delivery_date_gteq=2026-03-01&delivery_date_lteq=2026-03-31" +
      "&delivery_status_in[]=2&include_archive_flg=0&include_lost_flg=1&invoice_date_gteq=2026-04-01" +
      "&invoice_date_lteq=2026-04-30&invoice_timing_kbn_in[]=1&invoice_timing_kbn_in[]=2" +
      "&management_no_eq=M-1&name_cont=案件&order_status_in[]=1&order_status_in[]=4" +
      "&project_no_eq=P-1&response_group=all&tags[]=x",
  },
  {
    tool: "list_invoices",
    args: { project_client_id_eq: 3, invoice_status_in: [2, 5], response_group: "small" },
    path: "/v1/invoices",
    call: "GET /v1/invoices?invoice_status_in[]=2&invoice_status_in[]=5&project_client_id_eq=3&response_group=small",
  },
  {
    tool: "list_purchase_orders",
    args: {
      payee_id_eq: 5,
      project_id_eq: 6,
      expenditure_status_in: [5],
      delivery_status_in: [1, 2],
      response_group: "large",
    },
    path: "/v1/expenditures",
    call: "GET /v1/expenditures?delivery_status_in[]=1&delivery_status_in[]=2&expenditure_status_in[]=5&payee_id_eq=5&project_id_eq=6&response_group=large",
  },
  {
    // response_group を指定しない発注書の一覧は medium を送る(案件 project を含めるため)。
    tool: "list_purchase_orders",
    args: { payee_id_eq: 5 },
    path: "/v1/expenditures",
    call: "GET /v1/expenditures?payee_id_eq=5&response_group=medium",
  },
  {
    tool: "list_payments",
    args: {
      expenditure_payee_id_eq: 5,
      expenditure_expenditure_no_eq: 9,
      payment_status_in: [3],
      response_group: "large",
    },
    path: "/v1/expenditure_payments",
    call: "GET /v1/expenditure_payments?expenditure_expenditure_no_eq=9&expenditure_payee_id_eq=5&payment_status_in[]=3&response_group=large",
  },
  {
    tool: "list_vendors",
    args: { name_cont: "仕入" },
    path: "/v1/payees",
    call: "GET /v1/payees?name_cont=仕入",
  },
  {
    tool: "list_client_branches",
    args: { client_id: 2, include_archive_flg: true },
    path: "/v1/client_branches",
    call: "GET /v1/client_branches?client_id_eq=2&include_archive_flg=1",
  },
  {
    tool: "list_contacts",
    args: { client_id: 2 },
    path: "/v1/contacts",
    call: "GET /v1/contacts?client_id_eq=2",
  },
  {
    tool: "list_project_costs",
    args: { project_id: 11 },
    path: "/v1/project_costs",
    call: "GET /v1/project_costs?project_id_eq=11",
  },
  {
    tool: "list_vendor_branches",
    args: { payee_id_eq: 8 },
    path: "/v1/payee_branches",
    call: "GET /v1/payee_branches?payee_id_eq=8",
  },
  {
    tool: "list_vendor_contacts",
    args: { payee_id_eq: 8 },
    path: "/v1/payee_contacts",
    call: "GET /v1/payee_contacts?payee_id_eq=8",
  },
  {
    tool: "list_groups",
    args: { name_cont: "営業" },
    path: "/v1/groups",
    call: "GET /v1/groups?name_cont=営業",
  },
  {
    tool: "list_purchase_types",
    args: { name_cont: "外注", expenditure_type_kbn_eq: 2 },
    path: "/v1/expenditure_types",
    call: "GET /v1/expenditure_types?expenditure_type_kbn_eq=2&name_cont=外注",
  },
  {
    tool: "list_document_send_channels",
    // updated_at_* は仕様の一覧のクエリに無いので引数にも無い(issue #28)。include_archive_flg はある。
    args: { name_cont: "メール", include_archive_flg: true },
    path: "/v1/document_send_channels",
    call: "GET /v1/document_send_channels?include_archive_flg=1&name_cont=メール",
  },
  {
    tool: "list_project_types",
    args: { project_type_kbn_eq: 3 },
    path: "/v1/project_types",
    call: "GET /v1/project_types?project_type_kbn_eq=3",
  },
  {
    tool: "list_accounting_types",
    args: { accounting_type_kbn_eq: 2 },
    path: "/v1/accounting_types",
    call: "GET /v1/accounting_types?accounting_type_kbn_eq=2",
  },
  {
    tool: "list_projects",
    args: { response_group: "project_cost" },
    path: "/v1/projects",
    call: "GET /v1/projects?response_group=project_cost",
  },
  {
    tool: "list_projects",
    args: { response_group: "medium" },
    path: "/v1/projects",
    call: "GET /v1/projects?response_group=medium",
  },
];

describe("引数 → BOARD への問い合わせ(Go 版 CLI のフラグとの対応)", () => {
  it.each(QUERY_CASES)("$tool", async ({ tool, args, path, call }) => {
    const r = await callApiTool(tool, args, { [`GET ${BASE}${path}`]: ok([{ id: 1 }]) });
    expect(r.isError, r.text).toBe(false);
    expect(r.calls.map(describeApiCall)).toEqual([call]);
    expect(r.json.items).toEqual([{ id: 1 }]);
  });

  it("list_users の name_cont / email_cont はキャッシュ専用: 未同期なら絞り込みなしで取ってから絞る(API には送らない)", async () => {
    const rows = [
      userEntity({ id: 1, last_name: "山田", first_name: "太郎", email: "taro@example.com" }),
      userEntity({ id: 2, last_name: "鈴木", first_name: "花子", email: "hanako@example.com" }),
    ];
    const r = await callApiTool(
      "list_users",
      { name_cont: "太郎", email_cont: "@example" },
      { [`GET ${BASE}/v1/users`]: ok(rows) },
    );
    expect(r.isError, r.text).toBe(false);
    expect(r.calls.map(describeApiCall)).toEqual(["GET /v1/users"]);
    expect(r.json.items.map((u: { id: number }) => u.id)).toEqual([1]);
  });

  it("tags / *_in は配列。空配列は『指定なし』で、フィルタなし(ゼロフィルタ)として扱う", async () => {
    const r = await callApiTool("list_clients", { tags: [] });
    expect(r.isError, r.text).toBe(false);
    expect(r.calls).toHaveLength(0);
    expect(r.json).toEqual({ items: [], cache: [{ resource: "clients" }] });
  });
});

describe("list の出力", () => {
  it("cache が空でゼロフィルタ・refresh=none: API を呼ばず空の items と『未取得』の cache", async () => {
    const r = await callApiTool("list_clients", {});
    expect(r.json).toEqual({ items: [], cache: [{ resource: "clients" }] });
    expect(r.calls).toHaveLength(0);
  });

  it("show_meta=true のときだけ _meta を付ける(API から取った一覧)", async () => {
    const handler = async (call: Parameters<ReturnType<typeof pagedHandler>>[0]) => {
      const res = await pagedHandler(() => [{ id: 1 }, { id: 2 }], undefined, { totalHeader: 2 })(
        call,
      );
      res.headers.set("ETag", '"abc"');
      return res;
    };
    const key = { [`GET ${BASE}/v1/contacts`]: handler };
    const without = await callApiTool("list_contacts", { client_id: 1 }, key);
    expect(without.json).toEqual({ items: [{ id: 1 }, { id: 2 }], cache: [] });
    expect("_meta" in without.json).toBe(false);

    const withMeta = await callApiTool("list_contacts", { client_id: 1, show_meta: true }, key);
    expect(withMeta.json._meta).toMatchObject({ total_count: 2, etag: '"abc"' });
    expect(withMeta.json.items).toHaveLength(2);

    const explicitFalse = await callApiTool(
      "list_contacts",
      { client_id: 1, show_meta: false },
      key,
    );
    expect("_meta" in explicitFalse.json).toBe(false);
  });

  it("limit は件数を絞る(0 は無制限)", async () => {
    const key = { [`GET ${BASE}/v1/contacts`]: ok(makeEntities(5)) };
    const l2 = await callApiTool("list_contacts", { client_id: 1, limit: 2 }, key);
    expect(l2.json.items.map((e: { id: number }) => e.id)).toEqual([1, 2]);
    const l0 = await callApiTool("list_contacts", { client_id: 1, limit: 0 }, key);
    expect(l0.json.items).toHaveLength(5);
  });

  it("limit で切ったときだけ limited:true と total_before_limit を付ける(truncated とは別)", async () => {
    // 実際の BOARD は X-Total-Count を付ける。API 経路は limit に達したページで止めるので、全件数はこのヘッダーから分かる。
    const key = {
      [`GET ${BASE}/v1/contacts`]: pagedHandler(() => makeEntities(5), undefined, {
        totalHeader: true,
      }),
    };
    const cut = await callApiTool("list_contacts", { client_id: 1, limit: 2 }, key);
    expect(cut.json.limited).toBe(true);
    expect(cut.json.total_before_limit).toBe(5);
    expect("truncated" in cut.json).toBe(false);
    for (const limit of [5, 9, 0]) {
      const r = await callApiTool("list_contacts", { client_id: 1, limit }, key);
      expect("limited" in r.json, `limit=${limit}`).toBe(false);
      expect("total_before_limit" in r.json, `limit=${limit}`).toBe(false);
    }
    const unlimited = await callApiTool("list_contacts", { client_id: 1 }, key);
    expect("limited" in unlimited.json).toBe(false);
  });

  it("refresh=full でキャッシュを作り、refresh=none ではキャッシュから返す(cache に鮮度が載る)", async () => {
    const key = { [`GET ${BASE}/v1/clients`]: ok(makeEntities(3)) };
    const full = await callApiTool("list_clients", { refresh: "full" }, key);
    expect(full.json.items).toHaveLength(3);
    expect(full.json.cache[0]).toMatchObject({ resource: "clients", refresh_state: "complete" });
    expect(full.json.cache[0].cached_at).toBeTruthy();
    expect(full.calls).toHaveLength(1);

    const none = await callApiTool("list_clients", { refresh: "none" }, key);
    expect(none.json.items).toHaveLength(3);
    expect(none.calls).toHaveLength(0);
    expect(none.json.cache[0].full_refreshed_at).toBeTruthy();
  });

  it("nonzero-api: 非ゼロのフィルタがあると refresh は無視される(refresh しても API は 1 回、ロックも取らない)", async () => {
    const key = { [`GET ${BASE}/v1/contacts`]: ok([{ id: 1 }]) };
    const r = await callApiTool("list_contacts", { client_id: 1, refresh: "full" }, key);
    expect(r.calls).toHaveLength(1);
    expect(await getSyncState(testDb(), "contacts")).toBeNull();
  });

  it("BOARD の生データは BOARD のキーのまま返す", async () => {
    const row = { id: 1, name: "x", name_disp: "y", updated_at: "2026-01-01T00:00:00.000+09:00" };
    const r = await callApiTool(
      "list_clients",
      { name_cont: "x" },
      {
        [`GET ${BASE}/v1/clients`]: ok([row]),
      },
    );
    expect(r.json.items).toEqual([row]);
  });
});

describe("get の出力", () => {
  it("キャッシュにない: API から取り、2 回目はキャッシュ。show_meta で _meta", async () => {
    const handler = () => {
      const res = jsonResponse({ id: 5, name: "顧客5" });
      res.headers.set("ETag", '"e5"');
      return res;
    };
    const key = { [`GET ${BASE}/v1/clients/5`]: handler };
    const first = await callApiTool("get_client", { id: 5, show_meta: true }, key);
    expect(first.json).toMatchObject({ item: { id: 5 }, cache: [], _meta: { etag: '"e5"' } });
    expect(first.calls).toHaveLength(1);

    const second = await callApiTool("get_client", { id: 5, show_meta: true }, key);
    expect(second.calls).toHaveLength(0);
    expect(second.json.item).toEqual({ id: 5, name: "顧客5" });
    expect(second.json.cache[0].resource).toBe("clients");
    expect("_meta" in second.json).toBe(false);
  });

  it("書類(deliveries): 2 回目はキャッシュ、delta は何もしない、full は取り直す", async () => {
    const key = {
      [`GET ${BASE}/v1/documents/deliveries/9`]: () => jsonResponse({ id: 9, no: "D-9" }),
    };
    const a = await callApiTool("get_delivery", { id: 9 }, key);
    expect(a.calls).toHaveLength(1);
    const b = await callApiTool("get_delivery", { id: 9 }, key);
    expect(b.calls).toHaveLength(0);
    expect(b.json.item).toEqual({ id: 9, no: "D-9" });
    expect(b.json.cache[0]).toMatchObject({ resource: "deliveries" });
    const c = await callApiTool("get_delivery", { id: 9, refresh: "delta" }, key);
    expect(c.calls).toHaveLength(0);
    const d = await callApiTool("get_delivery", { id: 9, refresh: "full" }, key);
    expect(d.calls).toHaveLength(1);
  });

  it("get_project に response_group: 常に API を直接呼び(キャッシュ済みでも)、refresh は無視する", async () => {
    const key = {
      [`GET ${BASE}/v1/projects/3`]: () => jsonResponse({ id: 3, estimate: { id: 1 } }),
    };
    await callApiTool("get_project", { id: 3 }, key); // キャッシュに載せる
    const r = await callApiTool(
      "get_project",
      { id: 3, response_group: "estimate", refresh: "full" },
      key,
    );
    expect(r.calls.map(describeApiCall)).toEqual(["GET /v1/projects/3?response_group=estimate"]);
    expect(r.json.item).toEqual({ id: 3, estimate: { id: 1 } });
    expect(r.json.cache).toEqual([]);
  });

  it("BOARD が 404 を返すと not_found のエンベロープ", async () => {
    const r = await callApiTool(
      "get_user",
      { id: 99 },
      {
        [`GET ${BASE}/v1/users/99`]: () => jsonResponse({ message: "not found" }, 404),
      },
    );
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("not_found");
  });
});

describe("refresh_in_progress", () => {
  it("同じリソースの refresh が進行中なら、待たずに retry_after_seconds 付きのエンベロープ", async () => {
    await tryAcquireLock(testDb(), "clients");
    const r = await callApiTool("list_clients", { refresh: "delta" });
    expect(r.isError).toBe(true);
    expect(r.json).toMatchObject({
      schema_version: "1",
      error: { code: "refresh_in_progress", retryable: true, retry_after_seconds: 30 },
    });
    expect(r.calls).toHaveLength(0);
  });

  it("get でも同じ", async () => {
    await tryAcquireLock(testDb(), "clients");
    const r = await callApiTool("get_client", { id: 1, refresh: "full" });
    expect(r.json.error.code).toBe("refresh_in_progress");
    expect(r.json.error.retry_after_seconds).toBe(30);
  });
});

describe("呼び出しコンテキスト", () => {
  const tool = (name: string) => {
    const t = ALL_TOOLS.find((x) => x.name === name);
    if (!t) throw new Error(name);
    return t;
  };
  const toolCtx = (signal?: AbortSignal) => ({
    now: () => new Date(),
    signal,
    db: testDb,
    documentIndex: () => {
      throw new Error("api 層では使わない");
    },
    uiBaseUrl: "",
    repositories: () =>
      createRepositories({
        db: testDb(),
        client: createBoardClient({ baseUrl: BASE, apiKey: "k", apiToken: "t", maxRetries: 0 }),
      }),
  });

  it("MCP のリクエストの abort signal を repository(BOARD への fetch)まで渡す", async () => {
    const seen: (AbortSignal | undefined)[] = [];
    const api = mockOutboundFetch({
      [`GET ${BASE}/v1/contacts`]: () => new Promise<never>(() => {}),
    });
    const spy = api.spy;
    const controller = new AbortController();
    const running = tool("list_contacts").handler({ client_id: 1 }, toolCtx(controller.signal));
    const settled = running.then(
      () => "resolved",
      (e) => (e as Error).name,
    );
    // fetch が呼ばれるのを待ってから中断する。
    await vi.waitFor(() => expect(spy).toHaveBeenCalled());
    seen.push(spy.mock.calls[0]?.[1]?.signal ?? undefined);
    controller.abort();
    expect(await settled).toBe("BoardError");
    expect(seen[0]).toBeInstanceOf(AbortSignal);
  });

  it("締め切りは既定の 30 秒(ツール呼び出しごとに 1 つ作る)", async () => {
    const t0 = Date.now();
    const deadlines: number[] = [];
    const base = toolCtx();
    const repos = base.repositories();
    const spy = vi.spyOn(repos.contacts, "list").mockImplementation(async (_f, _o, ctx) => {
      deadlines.push(ctx?.deadline ?? 0);
      return { items: [], source: "api", cache: [] };
    });
    await tool("list_contacts").handler({}, { ...base, repositories: () => repos });
    expect(spy).toHaveBeenCalledTimes(1);
    expect(deadlines[0]).toBeGreaterThanOrEqual(t0 + 30_000);
    expect(deadlines[0]).toBeLessThan(t0 + 31_000);
  });

  it("締め切りは ctx.now() ではなく Date.now()(BOARD クライアントと同じ時計)から作る", async () => {
    const t0 = Date.now();
    const deadlines: number[] = [];
    const base = toolCtx();
    const repos = base.repositories();
    vi.spyOn(repos.contacts, "list").mockImplementation(async (_f, _o, ctx) => {
      deadlines.push(ctx?.deadline ?? 0);
      return { items: [], source: "api", cache: [] };
    });
    // ツールの時計が大きくずれていても、締め切りは本物の時計から 30 秒後。
    const skewed = {
      ...base,
      now: () => new Date("2000-01-01T00:00:00Z"),
      repositories: () => repos,
    };
    await tool("list_contacts").handler({}, skewed);
    expect(deadlines[0]).toBeGreaterThanOrEqual(t0 + 30_000);
    expect(deadlines[0]).toBeLessThan(t0 + 31_000);
  });
});
