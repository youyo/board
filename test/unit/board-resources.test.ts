import { afterEach, describe, expect, it, vi } from "vitest";
import { createBoardClient } from "../../src/board/client";
import * as R from "../../src/board/resources";
import { ArgumentError } from "../../src/errors";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";
import { OPENAPI_RESOURCES, type OpenApiResourceName } from "../helpers/openapi-fields";

const BASE = "https://api.board.test";

afterEach(() => {
  vi.restoreAllMocks();
});

const client = () =>
  createBoardClient({
    baseUrl: BASE,
    apiKey: "k",
    apiToken: "t",
    sleep: async () => {},
    maxRetries: 0,
  });

/** Go の XxxListOptions に共通するフィールド(updated_at と include_archive_flg)。 */
const COMMON_OPTS = {
  updatedAtGteq: "2024-01-01 00:00:00",
  updatedAtLteq: "2024-12-31 23:59:59",
  includeArchiveFlg: false,
};
const COMMON_PARAMS: Array<[string, string]> = [
  ["include_archive_flg", "0"],
  ["updated_at_gteq", "2024-01-01 00:00:00"],
  ["updated_at_lteq", "2024-12-31 23:59:59"],
];

type ListCase = {
  resource: string;
  path: string;
  // biome-ignore lint/suspicious/noExplicitAny: 22 種類の関数を表で扱うため
  build: (opts: any, page: number, perPage: number) => string;
  // biome-ignore lint/suspicious/noExplicitAny: 同上
  list: (c: ReturnType<typeof client>, opts?: any, paging?: any) => Promise<R.ListResult<unknown>>;
  /** 専用フィールドをすべて指定したときのオプション */
  opts: Record<string, unknown>;
  /** そのときに送るべきパラメーター(common 以外)。Go の build*Query と Go のテストの期待値 */
  params: Array<[string, string]>;
  /** 共通フィールドを持たない(Go の ListOptions にない)リソースの印 */
  noCommon?: boolean;
};

const NAME_ONLY = (
  resource: string,
  path: string,
  build: ListCase["build"],
  list: ListCase["list"],
  extra?: { opts: Record<string, unknown>; params: Array<[string, string]> },
): ListCase => ({
  resource,
  path,
  build,
  list,
  opts: { nameCont: "テスト", ...extra?.opts },
  params: [["name_cont", "テスト"], ...(extra?.params ?? [])],
});

const LIST_CASES: ListCase[] = [
  {
    resource: "clients",
    path: "/v1/clients",
    build: R.buildClientsQuery,
    list: R.listClients,
    opts: {
      nameCont: "ACME",
      nameDispCont: "ア",
      invoiceSystemNumberEq: "T1234567890123",
      customNoEq: "C-1",
      tags: ["vip", "", "new"],
      responseGroup: "large",
    },
    params: [
      ["name_cont", "ACME"],
      ["name_disp_cont", "ア"],
      ["invoice_system_number_eq", "T1234567890123"],
      ["custom_no_eq", "C-1"],
      ["tags[]", "vip"],
      ["tags[]", "new"],
      ["response_group", "large"],
    ],
  },
  {
    resource: "client_branches",
    path: "/v1/client_branches",
    build: R.buildClientBranchesQuery,
    list: R.listClientBranches,
    opts: { clientIdEq: 12345 },
    params: [["client_id_eq", "12345"]],
  },
  {
    resource: "contacts",
    path: "/v1/contacts",
    build: R.buildContactsQuery,
    list: R.listContacts,
    opts: { clientIdEq: 12345 },
    params: [["client_id_eq", "12345"]],
  },
  {
    resource: "projects",
    path: "/v1/projects",
    build: R.buildProjectsQuery,
    list: R.listProjects,
    opts: {
      createdAtGteq: "2023-01-01 00:00:00",
      createdAtLteq: "2023-12-31 23:59:59",
      includeLostFlg: true,
      nameCont: "テスト",
      clientIdEq: 12345,
      clientNameCont: "ACME",
      orderStatusIn: [1, 2],
      deliveryStatusIn: [3],
      projectNoEq: "P-001",
      managementNoEq: "M-9",
      deliveryDateGteq: "2024-01-01",
      deliveryDateLteq: "2024-12-31",
      invoiceDateGteq: "2024-02-01",
      invoiceDateLteq: "2024-11-30",
      invoiceTimingKbnIn: [2, 4],
      tags: ["a", "b"],
      responseGroup: "large",
    },
    params: [
      ["created_at_gteq", "2023-01-01 00:00:00"],
      ["created_at_lteq", "2023-12-31 23:59:59"],
      ["include_lost_flg", "1"],
      ["name_cont", "テスト"],
      ["client_id_eq", "12345"],
      ["client_name_cont", "ACME"],
      ["order_status_in[]", "1"],
      ["order_status_in[]", "2"],
      ["delivery_status_in[]", "3"],
      ["project_no_eq", "P-001"],
      ["management_no_eq", "M-9"],
      ["delivery_date_gteq", "2024-01-01"],
      ["delivery_date_lteq", "2024-12-31"],
      ["invoice_date_gteq", "2024-02-01"],
      ["invoice_date_lteq", "2024-11-30"],
      ["invoice_timing_kbn_in[]", "2"],
      ["invoice_timing_kbn_in[]", "4"],
      ["tags[]", "a"],
      ["tags[]", "b"],
      ["response_group", "large"],
    ],
  },
  {
    resource: "project_costs",
    path: "/v1/project_costs",
    build: R.buildProjectCostsQuery,
    list: R.listProjectCosts,
    opts: { projectIdEq: 555 },
    params: [["project_id_eq", "555"]],
  },
  {
    resource: "invoices",
    path: "/v1/invoices",
    build: R.buildInvoicesQuery,
    list: R.listInvoices,
    opts: { projectClientIdEq: 1, invoiceStatusIn: [2, 5], responseGroup: "small" },
    params: [
      ["project_client_id_eq", "1"],
      ["invoice_status_in[]", "2"],
      ["invoice_status_in[]", "5"],
      ["response_group", "small"],
    ],
  },
  {
    resource: "vendors",
    path: "/v1/payees",
    build: R.buildVendorsQuery,
    list: R.listVendors,
    opts: { nameCont: "仕入" },
    params: [["name_cont", "仕入"]],
  },
  {
    resource: "vendor_branches",
    path: "/v1/payee_branches",
    build: R.buildVendorBranchesQuery,
    list: R.listVendorBranches,
    opts: { payeeIdEq: 77 },
    params: [["payee_id_eq", "77"]],
  },
  {
    resource: "vendor_contacts",
    path: "/v1/payee_contacts",
    build: R.buildVendorContactsQuery,
    list: R.listVendorContacts,
    opts: { payeeIdEq: 77 },
    params: [["payee_id_eq", "77"]],
  },
  {
    resource: "purchase_orders",
    path: "/v1/expenditures",
    build: R.buildPurchaseOrdersQuery,
    list: R.listPurchaseOrders,
    opts: {
      payeeIdEq: 9,
      projectIdEq: 8,
      expenditureStatusIn: [5, 4],
      deliveryStatusIn: [2],
      responseGroup: "large",
    },
    params: [
      ["payee_id_eq", "9"],
      ["project_id_eq", "8"],
      ["expenditure_status_in[]", "5"],
      ["expenditure_status_in[]", "4"],
      ["delivery_status_in[]", "2"],
      ["response_group", "large"],
    ],
  },
  {
    resource: "payments",
    path: "/v1/expenditure_payments",
    build: R.buildPaymentsQuery,
    list: R.listPayments,
    opts: {
      expenditurePayeeIdEq: 9,
      paymentStatusIn: [3, 4],
      expenditureExpenditureNoEq: 6,
      responseGroup: "large",
    },
    params: [
      ["expenditure_payee_id_eq", "9"],
      ["payment_status_in[]", "3"],
      ["payment_status_in[]", "4"],
      ["expenditure_expenditure_no_eq", "6"],
      ["response_group", "large"],
    ],
  },
  {
    resource: "users",
    path: "/v1/users",
    build: R.buildUsersQuery,
    list: R.listUsers,
    // name_cont / email_cont はキャッシュ専用のキー。BOARD の API には送らない(送れるのは email_eq だけ)。
    opts: { nameCont: "立花", emailCont: "@board.example" },
    params: [],
  },
  NAME_ONLY("groups", "/v1/groups", R.buildGroupsQuery, R.listGroups),
  NAME_ONLY("payment_terms", "/v1/payment_terms", R.buildPaymentTermsQuery, R.listPaymentTerms),
  // 区分 1〜3 の選択(*_kbn_eq。未指定は 1 と同じ結果。issue #28)
  NAME_ONLY("project_types", "/v1/project_types", R.buildProjectTypesQuery, R.listProjectTypes, {
    opts: { projectTypeKbnEq: 2 },
    params: [["project_type_kbn_eq", "2"]],
  }),
  NAME_ONLY(
    "purchase_types",
    "/v1/expenditure_types",
    R.buildPurchaseTypesQuery,
    R.listPurchaseTypes,
    { opts: { expenditureTypeKbnEq: 3 }, params: [["expenditure_type_kbn_eq", "3"]] },
  ),
  NAME_ONLY(
    "accounting_types",
    "/v1/accounting_types",
    R.buildAccountingTypesQuery,
    R.listAccountingTypes,
    { opts: { accountingTypeKbnEq: 2 }, params: [["accounting_type_kbn_eq", "2"]] },
  ),
  NAME_ONLY(
    "document_send_channels",
    "/v1/document_send_channels",
    R.buildDocumentSendChannelsQuery,
    R.listDocumentSendChannels,
  ),
];

/** project_costs・invoices・... のうち、Go の ListOptions に共通フィールドがあるもの(全 18 リソース)。 */
const sorted = (pairs: Iterable<[string, string]>) =>
  [...pairs].sort((a, b) => (a[0] === b[0] ? 0 : a[0] < b[0] ? -1 : 1));
const sortedKeepOrder = (pairs: Array<[string, string]>) => sorted(pairs);

describe("22 リソースの一覧(Go の internal/boardapi と internal/cli/api_*.go の実態)", () => {
  it("リソース名・パス・list / get の有無", () => {
    expect(R.RESOURCES).toEqual([
      { name: "clients", path: "/v1/clients", list: true, get: true },
      { name: "client_branches", path: "/v1/client_branches", list: true, get: true },
      { name: "contacts", path: "/v1/contacts", list: true, get: true },
      { name: "projects", path: "/v1/projects", list: true, get: true },
      { name: "project_costs", path: "/v1/project_costs", list: true, get: true },
      { name: "estimates", path: "/v1/documents/estimates", list: false, get: true },
      { name: "invoices", path: "/v1/invoices", list: true, get: true },
      { name: "orders", path: "/v1/documents/orders", list: false, get: true },
      { name: "deliveries", path: "/v1/documents/deliveries", list: false, get: true },
      { name: "receipts", path: "/v1/documents/receipts", list: false, get: true },
      { name: "vendors", path: "/v1/payees", list: true, get: true },
      { name: "vendor_branches", path: "/v1/payee_branches", list: true, get: true },
      { name: "vendor_contacts", path: "/v1/payee_contacts", list: true, get: true },
      { name: "purchase_orders", path: "/v1/expenditures", list: true, get: true },
      { name: "payments", path: "/v1/expenditure_payments", list: true, get: true },
      { name: "users", path: "/v1/users", list: true, get: true },
      { name: "groups", path: "/v1/groups", list: true, get: true },
      { name: "payment_terms", path: "/v1/payment_terms", list: true, get: true },
      { name: "project_types", path: "/v1/project_types", list: true, get: true },
      { name: "purchase_types", path: "/v1/expenditure_types", list: true, get: true },
      { name: "accounting_types", path: "/v1/accounting_types", list: true, get: true },
      { name: "document_send_channels", path: "/v1/document_send_channels", list: true, get: true },
    ]);
    expect(R.RESOURCES).toHaveLength(22);
    expect(R.RESOURCES.filter((r) => r.list)).toHaveLength(18);
    expect(LIST_CASES.map((c) => c.resource).sort()).toEqual(
      R.RESOURCES.filter((r) => r.list)
        .map((r) => r.name)
        .sort(),
    );
  });
});

describe.each(LIST_CASES)("list $resource (GET $path)", (c) => {
  it("オプションなしは page と per_page だけ", () => {
    // 発注書の一覧だけは response_group 未指定で medium を送る(案件 project を含めるため。issue #27)
    const extra = c.resource === "purchase_orders" ? "&response_group=medium" : "";
    expect(c.build({}, 1, 100)).toBe(`page=1&per_page=100${extra}`);
  });

  it("Go の build*Query と同じパラメーターを送る", () => {
    const opts = { ...COMMON_OPTS, ...c.opts };
    const q = new URLSearchParams(c.build(opts, 2, 50));
    // 共通フィルタは、仕様(OpenAPI)の一覧のクエリにあるものだけ送る
    const spec = OPENAPI_RESOURCES[c.resource as OpenApiResourceName]
      .listQuery as readonly string[];
    const common = COMMON_PARAMS.filter(([k]) => spec.includes(k));
    expect(sorted(q.entries())).toEqual(
      sorted([["page", "2"], ["per_page", "50"], ...common, ...c.params]),
    );
  });

  it("includeArchiveFlg=true は 1(仕様に include_archive_flg があるリソースだけ)、未指定は送らない", () => {
    const spec = OPENAPI_RESOURCES[c.resource as OpenApiResourceName]
      .listQuery as readonly string[];
    const q = new URLSearchParams(c.build({ includeArchiveFlg: true }, 1, 10));
    expect(q.get("include_archive_flg")).toBe(spec.includes("include_archive_flg") ? "1" : null);
    expect(new URLSearchParams(c.build({}, 1, 10)).has("include_archive_flg")).toBe(false);
  });

  it("list はそのパスを page=1.. で呼び、items と _meta と truncated を返す", async () => {
    const m = mockOutboundFetch({
      [`GET ${BASE}${c.path}`]: () =>
        new Response(JSON.stringify([{ id: 1, extra_unknown: { deep: true } }]), {
          headers: { "X-Total-Count": "1", "X-Ratelimit-Remaining": "2999" },
        }),
    });
    const r = await c.list(client(), c.opts);
    expect(r.items).toEqual([{ id: 1, extra_unknown: { deep: true } }]);
    expect(r._meta).toEqual({ total_count: 1, rate_limit_remaining: 2999 });
    expect(r.truncated).toBe(false);
    expect(m.calls).toHaveLength(1);
    const url = new URL(m.calls[0]?.url ?? "");
    expect(url.pathname).toBe(c.path);
    expect(url.searchParams.get("page")).toBe("1");
    expect(url.searchParams.get("per_page")).toBe("100");
    expect(
      sorted([...url.searchParams.entries()].filter(([k]) => k !== "page" && k !== "per_page")),
    ).toEqual(sortedKeepOrder(c.params));
  });
});

describe("clients のクエリ文字列(Go の buildClientsQuery と同じ並び・エスケープ)", () => {
  it("すべて指定", () => {
    expect(
      R.buildClientsQuery(
        {
          ...COMMON_OPTS,
          nameCont: "エス",
          nameDispCont: "x y",
          invoiceSystemNumberEq: "T1",
          customNoEq: "C-1",
          tags: ["a", "b"],
          responseGroup: "large",
        },
        1,
        100,
      ),
    ).toBe(
      [
        "custom_no_eq=C-1",
        "include_archive_flg=0",
        "invoice_system_number_eq=T1",
        "name_cont=%E3%82%A8%E3%82%B9",
        "name_disp_cont=x+y",
        "page=1",
        "per_page=100",
        "response_group=large",
        "tags%5B%5D=a",
        "tags%5B%5D=b",
        "updated_at_gteq=2024-01-01+00%3A00%3A00",
        "updated_at_lteq=2024-12-31+23%3A59%3A59",
      ].join("&"),
    );
  });
});

describe("list の挙動", () => {
  it("opts.perPage / paging.maxPages が効く。上限で止めたら truncated", async () => {
    const m = mockOutboundFetch({
      [`GET ${BASE}/v1/groups`]: (call) => {
        const perPage = Number(new URL(call.url).searchParams.get("per_page"));
        return jsonResponse(Array.from({ length: perPage }, (_, i) => ({ id: i })));
      },
    });
    const r = await R.listGroups(client(), { perPage: 2 }, { maxPages: 2 });
    expect(r.items).toHaveLength(4);
    expect(r.truncated).toBe(true);
    expect(m.calls.map((x) => new URL(x.url).searchParams.get("page"))).toEqual(["1", "2"]);
    expect(m.calls.every((x) => new URL(x.url).searchParams.get("per_page") === "2")).toBe(true);
  });

  it("opts.page は無視する(Go では ListAllWithResult が page を上書きする)", async () => {
    const m = mockOutboundFetch({ [`GET ${BASE}/v1/groups`]: () => jsonResponse([]) });
    await R.listGroups(client(), { page: 9 });
    expect(new URL(m.calls[0]?.url ?? "").searchParams.get("page")).toBe("1");
  });
});

type GetCase = {
  resource: string;
  path: string;
  get: (c: ReturnType<typeof client>, id: number) => Promise<R.ItemResult<unknown>>;
};
const GET_CASES: GetCase[] = [
  { resource: "clients", path: "/v1/clients", get: R.getClient },
  { resource: "client_branches", path: "/v1/client_branches", get: R.getClientBranch },
  { resource: "contacts", path: "/v1/contacts", get: R.getContact },
  { resource: "projects", path: "/v1/projects", get: (c, id) => R.getProject(c, id) },
  { resource: "project_costs", path: "/v1/project_costs", get: R.getProjectCost },
  { resource: "estimates", path: "/v1/documents/estimates", get: R.getEstimate },
  { resource: "invoices", path: "/v1/invoices", get: R.getInvoice },
  { resource: "orders", path: "/v1/documents/orders", get: R.getOrder },
  { resource: "deliveries", path: "/v1/documents/deliveries", get: R.getDelivery },
  { resource: "receipts", path: "/v1/documents/receipts", get: R.getReceipt },
  { resource: "vendors", path: "/v1/payees", get: R.getVendor },
  { resource: "vendor_branches", path: "/v1/payee_branches", get: R.getVendorBranch },
  { resource: "vendor_contacts", path: "/v1/payee_contacts", get: R.getVendorContact },
  { resource: "purchase_orders", path: "/v1/expenditures", get: R.getPurchaseOrder },
  { resource: "payments", path: "/v1/expenditure_payments", get: R.getPayment },
  { resource: "users", path: "/v1/users", get: R.getUser },
  { resource: "groups", path: "/v1/groups", get: R.getGroup },
  { resource: "payment_terms", path: "/v1/payment_terms", get: R.getPaymentTerm },
  { resource: "project_types", path: "/v1/project_types", get: R.getProjectType },
  { resource: "purchase_types", path: "/v1/expenditure_types", get: R.getPurchaseType },
  { resource: "accounting_types", path: "/v1/accounting_types", get: R.getAccountingType },
  {
    resource: "document_send_channels",
    path: "/v1/document_send_channels",
    get: R.getDocumentSendChannel,
  },
];

describe.each(GET_CASES)("get $resource (GET $path/{id})", (c) => {
  it("{path}/{id} を呼び、本文をそのまま item にし、ヘッダーから _meta を作る", async () => {
    const m = mockOutboundFetch({
      [`GET ${BASE}${c.path}/*`]: () =>
        new Response(JSON.stringify({ id: 42, brand_new_field: [1, 2] }), {
          headers: { ETag: '"v1"', "X-Ratelimit-Limit": "3000", "X-Total-Count": "ignored" },
        }),
    });
    const r = await c.get(client(), 42);
    expect(r.item).toEqual({ id: 42, brand_new_field: [1, 2] });
    expect(r._meta).toEqual({ etag: '"v1"', rate_limit_limit: 3000 });
    expect(m.calls).toHaveLength(1);
    expect(m.calls[0]?.url).toBe(`${BASE}${c.path}/42`);
  });

  it("id は 1 以上の整数だけ(パスの組み立てに使うため)", async () => {
    const m = mockOutboundFetch({});
    for (const bad of [0, -1, 1.5, Number.NaN, Number.POSITIVE_INFINITY, 2 ** 60]) {
      await expect(c.get(client(), bad)).rejects.toBeInstanceOf(ArgumentError);
    }
    expect(m.calls).toHaveLength(0);
  });
});

describe("get project (response_group)", () => {
  it("response_group を付けられる。空なら付けない (Go の GetProjectWithGroup)", async () => {
    const m = mockOutboundFetch({ [`GET ${BASE}/v1/projects/*`]: () => jsonResponse({ id: 5 }) });
    await R.getProject(client(), 5, { responseGroup: "all" });
    await R.getProject(client(), 5, { responseGroup: "" });
    await R.getProject(client(), 5);
    expect(m.calls.map((x) => x.url)).toEqual([
      `${BASE}/v1/projects/5?response_group=all`,
      `${BASE}/v1/projects/5`,
      `${BASE}/v1/projects/5`,
    ]);
  });
});
