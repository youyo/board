import { env } from "cloudflare:workers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { clearBoardClientCache } from "../../src/board/client";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { BASE, pagedHandler } from "../helpers/board-list";
import { resetCache } from "../helpers/d1";
import { firstText, readRpc } from "../helpers/mcp";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";
import { mcpCall } from "../helpers/worker";

// find 層(M05)を Worker 全体(Access の JWT 検証 → /mcp → ツール → find → repository → D1 / 本物の BOARD クライアント)で通す。
// BOARD API は outbound fetch のモック。実 BOARD には通信しない。UI のベース URL は BOARD_BASE_URL から導く(https://board.test)。

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;
const SECRETS = ["test-board-api-key", "test-board-api-token"];
const UI = "https://board.test";

let key: TestKey;
let jwt: string;

beforeEach(async () => {
  clearJwksCache();
  clearBoardClientCache();
  await resetCache(env.DB);
  key = await createTestKey("kid-1");
  jwt = await signAccessJwt(key);
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  clearJwksCache();
  clearBoardClientCache();
});

function boardMocks(handlers: Parameters<typeof mockOutboundFetch>[0]) {
  return mockOutboundFetch({ [CERTS]: () => jwksResponse(key), ...handlers });
}
const boardCalls = (m: ReturnType<typeof mockOutboundFetch>) =>
  m.calls.filter((c) => c.url.startsWith(BASE));
const paths = (m: ReturnType<typeof mockOutboundFetch>) =>
  boardCalls(m).map((c) => new URL(c.url).pathname);

async function tool(name: string, args: Record<string, unknown>, id = 1) {
  const res = await readRpc(await mcpCall(jwt, "tools/call", { name, arguments: args }, id));
  const text = firstText(res);
  return { isError: res.result?.isError === true, text, json: JSON.parse(text) };
}

const get = (p: string) => `GET ${BASE}${p}`;
const one = (body: unknown) => () => jsonResponse(body);
const list = (rows: unknown[]) => pagedHandler(() => rows);

describe("E2E: tools/list", () => {
  it("find 層 11 個が読み取り専用で出る(合計 52 個)", async () => {
    boardMocks({});
    const res = await readRpc(await mcpCall(jwt, "tools/list"));
    const tools = res.result?.tools ?? [];
    expect(tools).toHaveLength(52);
    const find = tools.filter((t) => t.name.startsWith("find_")).map((t) => t.name);
    expect(find.sort()).toEqual(
      [
        "find_clients",
        "find_vendors",
        "find_users",
        "find_projects",
        "find_estimates",
        "find_invoices",
        "find_orders",
        "find_deliveries",
        "find_receipts",
        "find_purchase_orders",
        "find_payments",
      ].sort(),
    );
    for (const t of tools) expect(t.annotations?.readOnlyHint).toBe(true);
  });
});

describe("E2E: 11 ツールの代表的な成功パス", () => {
  it("find_clients: 名前で検索 → 支店・担当者・url が付く。cache に clients の鮮度が載る", async () => {
    const m = boardMocks({
      [get("/v1/clients")]: list([
        { id: 1, name: "株式会社A", updated_at: "2026-01-01T00:00:00+09:00" },
      ]),
      [get("/v1/client_branches")]: list([{ id: 11, name: "本社" }]),
      [get("/v1/contacts")]: list([{ id: 21, name: "山田" }]),
    });
    const r = await tool("find_clients", { name: "株式会社", refresh: "full" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toEqual([
      {
        Client: expect.objectContaining({ id: 1, name: "株式会社A" }),
        Branches: [{ id: 11, name: "本社" }],
        Contacts: [{ id: 21, name: "山田" }],
        url: `${UI}/clients/1/edit`,
      },
    ]);
    // cache は読んだリソース(キャッシュから返した clients)だけ。支店・担当者は API 直取りなので載らない。
    expect(r.json.cache).toHaveLength(1);
    expect(r.json.cache[0]).toMatchObject({ resource: "clients", refresh_state: "complete" });
    expect(r.json.cache[0].cached_at).toBeTruthy();
    expect(r.json.cache[0].full_refreshed_at).toBeTruthy();
    expect("warnings" in r.json).toBe(false);
    const q = boardCalls(m).find((c) => c.url.includes("/v1/client_branches"));
    expect(new URL(q?.url ?? "").searchParams.get("client_id_eq")).toBe("1");
  });

  it("find_vendors: id → 仕入先 + 支店・担当者(payee_id_eq)+ payees の url", async () => {
    const m = boardMocks({
      [get("/v1/payees/5")]: one({ id: 5, name: "仕入先" }),
      [get("/v1/payee_branches")]: list([{ id: 51 }]),
      [get("/v1/payee_contacts")]: list([]),
    });
    const r = await tool("find_vendors", { id: 5 });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0]).toEqual({
      Vendor: { id: 5, name: "仕入先" },
      Branches: [{ id: 51 }],
      Contacts: [],
      url: `${UI}/payees/5/edit`,
    });
    const q = boardCalls(m).find((c) => c.url.includes("/v1/payee_branches"));
    expect(new URL(q?.url ?? "").searchParams.get("payee_id_eq")).toBe("5");
  });

  it("find_users: 名前で検索", async () => {
    boardMocks({ [get("/v1/users")]: list([{ id: 3, name: "田中" }]) });
    const r = await tool("find_users", { name: "田中" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toEqual([{ User: { id: 3, name: "田中" } }]);
  });

  it("find_projects: client_name を解決 → 案件を client_id_eq で検索 → 顧客が付く。contract_status で絞る", async () => {
    const m = boardMocks({
      [get("/v1/clients")]: list([{ id: 7, name: "アクメ" }]),
      [get("/v1/clients/7")]: one({ id: 7, name: "アクメ" }),
      [get("/v1/projects")]: list([
        {
          id: 1,
          name: "保守A",
          client: { id: 7 },
          delivery_status_name: "着手中",
          order_status_name: "受注済",
        },
        {
          id: 2,
          name: "保守B",
          client: { id: 7 },
          delivery_status_name: "検収済",
          order_status_name: "受注済",
        },
      ]),
    });
    const r = await tool("find_projects", {
      client_name: "アクメ",
      name: "保守",
      contract_status: "active",
    });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toHaveLength(1);
    expect(r.json.items[0]).toMatchObject({
      Project: { id: 1 },
      Client: { id: 7, name: "アクメ" },
      url: `${UI}/projects/1/edit`,
    });
    const q = boardCalls(m).find((c) => c.url.includes("/v1/projects"));
    expect(new URL(q?.url ?? "").searchParams.get("client_id_eq")).toBe("7");
    expect(new URL(q?.url ?? "").searchParams.get("name_cont")).toBe("保守");
  });

  it("find_estimates: id → 書類 + 逆引きで案件・顧客(project_id から url)", async () => {
    boardMocks({
      [get("/v1/documents/estimates/500")]: one({ id: 500, total: "100" }),
      [get("/v1/projects")]: list([
        { id: 42, name: "案件", client: { id: 7 }, estimate: { id: 500 } },
      ]),
      [get("/v1/projects/42")]: one({ id: 42, name: "案件", client: { id: 7 } }),
      [get("/v1/clients/7")]: one({ id: 7, name: "顧客" }),
    });
    const r = await tool("find_estimates", { id: 500 });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toEqual([
      {
        Estimate: { id: 500, total: "100" },
        ProjectID: 42,
        ClientID: 7,
        Project: { id: 42, name: "案件", client: { id: 7 } },
        Client: { id: 7, name: "顧客" },
        url: `${UI}/documents/42/edit`,
      },
    ]);
    expect("warnings" in r.json).toBe(false);
  });

  it("find_invoices: client_name を解決 → 請求書 + 顧客 + 案件", async () => {
    boardMocks({
      [get("/v1/clients")]: list([{ id: 7, name: "A" }]),
      [get("/v1/clients/7")]: one({ id: 7, name: "A" }),
      [get("/v1/projects/42")]: one({ id: 42, name: "案件" }),
      [get("/v1/invoices")]: list([{ id: 9, client_id: 7, project_id: 42, status: "sent" }]),
    });
    const r = await tool("find_invoices", { client_name: "A", status: "sent" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0]).toMatchObject({
      Invoice: { id: 9 },
      Client: { id: 7 },
      Project: { id: 42 },
      url: `${UI}/documents/42/edit`,
    });
  });

  it("find_orders / find_deliveries / find_receipts: project_id から書類(納品・領収は配列の全件)", async () => {
    boardMocks({
      [get("/v1/projects/42")]: (call) => {
        const rg = new URL(call.url).searchParams.get("response_group");
        return jsonResponse({
          id: 42,
          name: "案件",
          client: { id: 7 },
          order: rg === "order" ? { id: 600 } : undefined,
          deliveries: rg === "delivery" ? [{ id: 700 }, { id: 701 }] : undefined,
          receipts: rg === "receipt" ? [{ id: 800 }] : undefined,
        });
      },
      [get("/v1/clients/7")]: one({ id: 7, name: "顧客" }),
      [get("/v1/documents/orders/600")]: one({ id: 600 }),
      [get("/v1/documents/deliveries/700")]: one({ id: 700 }),
      [get("/v1/documents/deliveries/701")]: one({ id: 701 }),
      [get("/v1/documents/receipts/800")]: one({ id: 800 }),
    });
    const o = await tool("find_orders", { project_id: 42 });
    expect(o.json.items).toHaveLength(1);
    expect(o.json.items[0]).toMatchObject({ Order: { id: 600 }, ProjectID: 42, ClientID: 7 });
    const d = await tool("find_deliveries", { project_id: 42 }, 2);
    expect(d.json.items.map((x: { Delivery: { id: number } }) => x.Delivery.id)).toEqual([
      700, 701,
    ]);
    const rc = await tool("find_receipts", { project_id: 42 }, 3);
    expect(rc.json.items[0]).toMatchObject({
      Receipt: { id: 800 },
      url: `${UI}/documents/42/edit`,
    });
  });

  it("find_purchase_orders: vendor_name を解決 → 発注書 + 仕入先 + 案件", async () => {
    boardMocks({
      [get("/v1/payees")]: list([{ id: 3, name: "仕入先A" }]),
      [get("/v1/payees/3")]: one({ id: 3, name: "仕入先A" }),
      [get("/v1/projects/42")]: one({ id: 42, name: "案件" }),
      [get("/v1/expenditures")]: list([{ id: 5, vendor_id: 3, project_id: 42, status: "open" }]),
    });
    const r = await tool("find_purchase_orders", { vendor_name: "仕入先" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0]).toMatchObject({
      PurchaseOrder: { id: 5 },
      Vendor: { id: 3 },
      Project: { id: 42 },
    });
  });

  it("find_payments: status で検索 → 仕入先が付き、Project は null・url なし", async () => {
    boardMocks({
      [get("/v1/expenditure_payments")]: list([{ id: 8, vendor_id: 3, status: "paid" }]),
      [get("/v1/payees/3")]: one({ id: 3, name: "仕入先A" }),
    });
    const r = await tool("find_payments", { status: "paid" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toEqual([
      {
        Payment: { id: 8, vendor_id: 3, status: "paid" },
        Vendor: { id: 3, name: "仕入先A" },
        Project: null,
      },
    ]);
  });
});

describe("E2E: 名前の曖昧さと fanout", () => {
  it("find_projects: client_name が複数にマッチ → argument_error、候補は 5 件まで。案件の検索には進まない", async () => {
    const rows = Array.from({ length: 7 }, (_, i) => ({ id: i + 1, name: `会社${i + 1}` }));
    const m = boardMocks({ [get("/v1/clients")]: list(rows) });
    const r = await tool("find_projects", { client_name: "会社" });
    expect(r.isError).toBe(true);
    expect(r.json.error).toMatchObject({ code: "argument_error", retryable: false });
    expect(r.json.error.message).toContain('multiple clients match name "会社" (7 hits)');
    expect(r.json.error.message).toContain("id=5");
    expect(r.json.error.message).not.toContain("id=6");
    expect(r.json.error.message).toContain("... (2 more)");
    expect(paths(m)).toEqual(["/v1/clients"]);
  });

  it("find_estimates: client_name は一致した全顧客に fanout する", async () => {
    const m = boardMocks({
      [get("/v1/clients")]: list([
        { id: 1, name: "A社" },
        { id: 2, name: "A社2" },
      ]),
      [get("/v1/projects")]: (call) => {
        const cid = new URL(call.url).searchParams.get("client_id_eq");
        return jsonResponse([
          {
            id: Number(cid) * 10,
            name: "案件",
            client: { id: Number(cid) },
            estimate: { id: Number(cid) * 100 },
          },
        ]);
      },
      [get("/v1/documents/estimates/*")]: (call) =>
        jsonResponse({ id: Number(call.url.split("/").pop()) }),
    });
    const r = await tool("find_estimates", { client_name: "A社" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items.map((x: { ProjectID: number }) => x.ProjectID)).toEqual([10, 20]);
    expect(r.json.items[1].Client).toMatchObject({ id: 2 });
    const projectCalls = boardCalls(m).filter((c) => c.url.includes("/v1/projects"));
    expect(projectCalls.map((c) => new URL(c.url).searchParams.get("response_group"))).toEqual([
      "estimate",
      "estimate",
    ]);
    // fanout した案件の書類 ID は逆引きに覚える。
    const mapped = await env.DB.prepare(
      "SELECT document_id, project_id FROM document_project_map WHERE document_kind = 'estimate' ORDER BY document_id",
    ).all();
    expect(mapped.results).toEqual([
      { document_id: 100, project_id: 10 },
      { document_id: 200, project_id: 20 },
    ]);
  });

  it("find_estimates: client_name が 5 件を超えてマッチしたら拒否する(案件にも書類にも進まない)", async () => {
    const rows = Array.from({ length: 6 }, (_, i) => ({ id: i + 1, name: `A社${i + 1}` }));
    const m = boardMocks({ [get("/v1/clients")]: list(rows) });
    const r = await tool("find_estimates", { client_name: "A社" });
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain('client_name "A社" matched 6 entities (cap=5)');
    expect(paths(m)).toEqual(["/v1/clients"]);
  });
});

describe("E2E: 逆引き(書類 ID → 案件 ID)の縮退", () => {
  /** 案件 n 件。案件 i の見積 ID = 1000 + i。 */
  const projects = (n: number) =>
    Array.from({ length: n }, (_, i) => ({
      id: i + 1,
      name: `案件${i + 1}`,
      client: { id: 7 },
      estimate: { id: 1000 + i + 1 },
    }));

  it("走査が締め切りの手前で止まったら、書類は返し ProjectID=0 + 警告。進捗が残り、次の呼び出しが続きから解決する", async () => {
    // Date だけを偽装する(setTimeout は本物)。1 ページ取るたびに 10 秒進める。
    vi.useFakeTimers({ toFake: ["Date"] });
    let slow = true;
    const m = boardMocks({
      [get("/v1/documents/estimates/1250")]: one({ id: 1250, total: "1" }),
      [get("/v1/projects")]: pagedHandler(
        () => projects(300),
        () => {
          if (slow) vi.setSystemTime(Date.now() + 10_000);
          return undefined;
        },
      ),
      [get("/v1/projects/250")]: one({ id: 250, name: "案件250", client: { id: 7 } }),
      [get("/v1/clients/7")]: one({ id: 7, name: "顧客" }),
    });

    const t0 = Date.now();
    const first = await tool("find_estimates", { id: 1250 });
    expect(first.isError, first.text).toBe(false);
    expect(first.json.items).toEqual([
      {
        Estimate: { id: 1250, total: "1" },
        ProjectID: 0,
        ClientID: 0,
        Project: null,
        Client: null,
      },
    ]);
    expect(first.json.warnings).toEqual([
      expect.objectContaining({
        code: "project_unresolved",
        resource: "estimates",
        document_id: 1250,
        reason: "deadline",
      }),
    ]);
    // 締め切り(30 秒)の手前で返っている: 走査は予約(8 秒)を残して止まる。
    expect(Date.now() - t0).toBeLessThan(30_000 - 8_000 + 10_000);
    const scanned = paths(m).filter((p) => p === "/v1/projects").length;
    expect(scanned).toBeGreaterThanOrEqual(1);
    expect(scanned).toBeLessThanOrEqual(3);
    const state = await env.DB.prepare(
      "SELECT next_page, completed_at, lock_owner FROM document_scan_state WHERE document_kind = 'estimate'",
    ).first<{ next_page: number; completed_at: string | null; lock_owner: string | null }>();
    expect(state?.next_page).toBe(scanned + 1);
    expect(state?.completed_at).toBeNull();
    expect(state?.lock_owner).toBeNull();

    // 2 回目: 遅くなくなった。保存した次のページから続けて案件 250 に届き、解決する。
    slow = false;
    const before = paths(m).filter((p) => p === "/v1/projects").length;
    const second = await tool("find_estimates", { id: 1250 }, 2);
    expect(second.isError, second.text).toBe(false);
    expect(second.json.items[0]).toMatchObject({
      ProjectID: 250,
      ClientID: 7,
      Project: { id: 250 },
      Client: { id: 7 },
      url: `${UI}/documents/250/edit`,
    });
    expect("warnings" in second.json).toBe(false);
    const secondFirstPage = boardCalls(m)
      .filter((c) => new URL(c.url).pathname === "/v1/projects")
      .at(before)?.url;
    expect(new URL(secondFirstPage ?? "").searchParams.get("page")).toBe(String(scanned + 1));

    // 3 回目: 表にあるので、projects の走査はしない。
    const callsBefore = boardCalls(m).length;
    const third = await tool("find_estimates", { id: 1250 }, 3);
    expect(third.json.items[0].ProjectID).toBe(250);
    expect(
      boardCalls(m)
        .slice(callsBefore)
        .filter((c) => new URL(c.url).pathname === "/v1/projects"),
    ).toHaveLength(0);
  });

  it("最後まで見ても無い書類は ProjectID=0 + not_found の警告(再走査はしばらくしない)", async () => {
    const m = boardMocks({
      [get("/v1/documents/estimates/9999")]: one({ id: 9999 }),
      [get("/v1/projects")]: list(projects(3)),
    });
    const first = await tool("find_estimates", { id: 9999 });
    expect(first.json.items[0].ProjectID).toBe(0);
    expect(first.json.warnings[0]).toMatchObject({
      code: "project_unresolved",
      reason: "not_found",
    });
    const scanned = paths(m).filter((p) => p === "/v1/projects").length;
    const second = await tool("find_estimates", { id: 9999 }, 2);
    expect(second.json.warnings[0]).toMatchObject({ reason: "not_found" });
    expect(paths(m).filter((p) => p === "/v1/projects")).toHaveLength(scanned);
  });
});

describe("E2E: limit・cache・警告", () => {
  it("limit で切ったときだけ limited:true と total_before_limit が付く", async () => {
    boardMocks({
      [get("/v1/users")]: list([
        { id: 1, name: "田中A" },
        { id: 2, name: "田中B" },
      ]),
    });
    const cut = await tool("find_users", { name: "田中", limit: 1 });
    expect(cut.json.items).toHaveLength(1);
    expect(cut.json).toMatchObject({ limited: true, total_before_limit: 2 });
    const all = await tool("find_users", { name: "田中", limit: 2 }, 2);
    expect("limited" in all.json).toBe(false);
    expect("total_before_limit" in all.json).toBe(false);
  });

  it("補助情報の取得失敗は致命的でない: null + 警告(エラーコードだけ。BOARD の本文は載せない)", async () => {
    boardMocks({
      [get("/v1/clients/1")]: one({ id: 1, name: "A" }),
      [get("/v1/client_branches")]: () =>
        jsonResponse({ message: `bad key test-board-api-key / token test-board-api-token` }, 500),
      [get("/v1/contacts")]: list([{ id: 2 }]),
    });
    const r = await tool("find_clients", { id: 1 });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0]).toMatchObject({ Branches: null, Contacts: [{ id: 2 }] });
    expect(r.json.warnings).toEqual([
      { code: "enrichment_failed", resource: "client_branches", id: 1, error_code: "api_error" },
    ]);
  });
});

describe("E2E: 秘密情報", () => {
  it("正常・縮退・警告・エラーのどの応答にも API キー・トークン・JWT が出ない。D1 の全テーブルにも入らない", async () => {
    boardMocks({
      [get("/v1/clients/1")]: one({ id: 1, name: "A" }),
      [get("/v1/client_branches")]: () =>
        jsonResponse({ message: "x test-board-api-key y test-board-api-token" }, 500),
      [get("/v1/contacts")]: list([]),
      [get("/v1/documents/estimates/77")]: one({ id: 77 }),
      [get("/v1/projects")]: () =>
        jsonResponse({ message: "boom test-board-api-key test-board-api-token" }, 500),
      [get("/v1/users/2")]: () =>
        jsonResponse({ message: "invalid test-board-api-key test-board-api-token" }, 401),
    });
    const responses = [
      await tool("find_clients", { id: 1 }),
      await tool("find_estimates", { id: 77 }, 2),
      await tool("find_users", { id: 2 }, 3),
      await tool("find_clients", { bogus: 1 }, 4),
      await tool("find_payments", { purchase_order_id: 3 }, 5),
    ];
    expect(responses[1]?.json.warnings[0]).toMatchObject({
      reason: "api_error",
      error_code: "api_error",
    });
    expect(responses[2]?.isError).toBe(true);
    for (const r of responses) {
      for (const secret of [...SECRETS, jwt]) expect(r.text).not.toContain(secret);
    }
    for (const table of [
      "resource_cache",
      "sync_state",
      "cache_meta",
      "document_project_map",
      "document_scan_state",
    ]) {
      const rows = await env.DB.prepare(`SELECT * FROM ${table}`).all();
      const dump = JSON.stringify(rows.results);
      for (const secret of [...SECRETS, jwt]) expect(dump).not.toContain(secret);
    }
  });
});
