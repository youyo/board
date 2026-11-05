import { describe, expect, it } from "vitest";
import { z } from "zod";
import { createMcpServer } from "../../src/mcp/server";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { API_RESOURCES } from "../../src/mcp/tools/api-resources";
import { SPECS } from "../../src/repository";
import { callApiTool } from "../helpers/api-tool";
import { rpcTo } from "../helpers/mcp";

/** 40 ツールの名前(plans/board-workers-m04-api-tools.md の一覧と同じ。自動の単数化はしない)。 */
const LIST_TOOLS: Record<string, string> = {
  clients: "list_clients",
  client_branches: "list_client_branches",
  contacts: "list_contacts",
  projects: "list_projects",
  project_costs: "list_project_costs",
  invoices: "list_invoices",
  vendors: "list_vendors",
  vendor_branches: "list_vendor_branches",
  vendor_contacts: "list_vendor_contacts",
  purchase_orders: "list_purchase_orders",
  payments: "list_payments",
  users: "list_users",
  groups: "list_groups",
  payment_terms: "list_payment_terms",
  project_types: "list_project_types",
  purchase_types: "list_purchase_types",
  accounting_types: "list_accounting_types",
  document_send_channels: "list_document_send_channels",
};
const GET_TOOLS: Record<string, string> = {
  clients: "get_client",
  client_branches: "get_client_branch",
  contacts: "get_contact",
  projects: "get_project",
  project_costs: "get_project_cost",
  estimates: "get_estimate",
  invoices: "get_invoice",
  orders: "get_order",
  deliveries: "get_delivery",
  receipts: "get_receipt",
  vendors: "get_vendor",
  vendor_branches: "get_vendor_branch",
  vendor_contacts: "get_vendor_contact",
  purchase_orders: "get_purchase_order",
  payments: "get_payment",
  users: "get_user",
  groups: "get_group",
  payment_terms: "get_payment_term",
  project_types: "get_project_type",
  purchase_types: "get_purchase_type",
  accounting_types: "get_accounting_type",
  document_send_channels: "get_document_send_channel",
};

const COMMON_FILTERS = ["updated_at_gteq", "updated_at_lteq", "include_archive_flg"];
const COMMON_ARGS = ["refresh", "limit", "show_meta", "max_response_tokens"];

/**
 * Go 版 CLI の list のフィルタフラグ(`internal/cli/api_*.go` の `cmd.Flags()`、ハイフンを `_` にしたもの)。
 * 共通の updated-at-gteq / updated-at-lteq / include-archive-flg は COMMON_FILTERS で足す。
 */
const GO_LIST_FLAGS: Record<string, string[]> = {
  clients: [
    "name_cont",
    "name_disp_cont",
    "invoice_system_number_eq",
    "custom_no_eq",
    "tags",
    "response_group",
  ],
  client_branches: ["client_id", "name_cont"],
  contacts: ["client_id", "name_cont", "email_cont"],
  projects: [
    "name_cont",
    "client_id",
    "client_name_cont",
    "order_status_in",
    "delivery_status_in",
    "project_no_eq",
    "management_no_eq",
    "delivery_date_gteq",
    "delivery_date_lteq",
    "invoice_date_gteq",
    "invoice_date_lteq",
    "invoice_timing_kbn_in",
    "tags",
    "response_group",
    "created_at_gteq",
    "created_at_lteq",
    "include_lost_flg",
  ],
  project_costs: ["project_id"],
  invoices: ["client_id_eq", "project_id_eq", "status_eq", "response_group"],
  vendors: ["name_cont"],
  vendor_branches: ["payee_id_eq", "name_cont"],
  vendor_contacts: ["payee_id_eq", "name_cont", "email_cont"],
  purchase_orders: ["vendor_id_eq", "project_id_eq", "status_eq", "response_group"],
  payments: ["vendor_id_eq", "purchase_order_id_eq", "status_eq", "response_group"],
  users: ["name_cont", "email_cont"],
  groups: ["name_cont"],
  payment_terms: ["name_cont"],
  project_types: ["name_cont"],
  purchase_types: ["name_cont"],
  accounting_types: ["name_cont"],
  document_send_channels: ["name_cont"],
};

const NONZERO_API = [
  "client_branches",
  "contacts",
  "project_costs",
  "vendor_branches",
  "vendor_contacts",
  "groups",
  "payment_terms",
  "project_types",
  "purchase_types",
  "accounting_types",
  "document_send_channels",
];
const DOCUMENTS = ["estimates", "orders", "deliveries", "receipts"];

const byName = new Map(ALL_TOOLS.map((t) => [t.name, t]));
const props = (name: string) =>
  Object.keys(
    (
      z.toJSONSchema(byName.get(name)?.inputSchema as z.ZodObject) as {
        properties: Record<string, unknown>;
      }
    ).properties,
  ).sort();

describe("api 層のツール一覧", () => {
  it("server_info + list 18 + get 22 = 41 個", () => {
    expect(Object.keys(LIST_TOOLS)).toHaveLength(18);
    expect(Object.keys(GET_TOOLS)).toHaveLength(22);
    expect(ALL_TOOLS.map((t) => t.name).sort()).toEqual(
      ["server_info", ...Object.values(LIST_TOOLS), ...Object.values(GET_TOOLS)].sort(),
    );
  });

  it("すべて ReadOnly で、接頭辞はなく、title と description がある", () => {
    for (const name of [...Object.values(LIST_TOOLS), ...Object.values(GET_TOOLS)]) {
      const t = byName.get(name);
      expect(t?.category, name).toBe("ReadOnly");
      expect(t?.responseLimit, name).toBe(true);
      expect(t?.annotations.title.length, name).toBeGreaterThan(0);
      expect(t?.description.length, name).toBeGreaterThan(20);
      expect(name.startsWith("board_"), name).toBe(false);
    }
  });
});

describe("list の引数は Go 版 CLI のフィルタフラグと一致する", () => {
  it.each(Object.entries(LIST_TOOLS))("%s → %s", (resource, tool) => {
    const expected = [...(GO_LIST_FLAGS[resource] ?? []), ...COMMON_FILTERS, ...COMMON_ARGS].sort();
    expect(props(tool)).toEqual([...new Set(expected)].sort());
  });

  it("nonzero-api の 11 リソースは、非ゼロのフィルタで refresh が無視されることを description に書く", () => {
    for (const resource of NONZERO_API) {
      const d = byName.get(LIST_TOOLS[resource] as string)?.description ?? "";
      expect(d, resource).toContain("refresh は無視");
    }
  });

  it("cache-first のリソースは、絞り込み項目によって API を直接呼ぶことを description に書く", () => {
    for (const resource of [
      "clients",
      "projects",
      "invoices",
      "purchase_orders",
      "payments",
      "users",
      "vendors",
    ]) {
      const d = byName.get(LIST_TOOLS[resource] as string)?.description ?? "";
      expect(d, resource).toContain("API を直接");
    }
  });

  it("response_group の列挙値は Go に合わせる(projects は 8 種、他は small / large)", () => {
    const rg = (tool: string) =>
      (
        z.toJSONSchema(byName.get(tool)?.inputSchema as z.ZodObject) as {
          properties: Record<string, { enum?: string[] }>;
        }
      ).properties.response_group?.enum;
    expect(rg("list_projects")).toEqual([
      "small",
      "large",
      "estimate",
      "order",
      "delivery",
      "invoice",
      "receipt",
      "all",
    ]);
    for (const t of ["list_clients", "list_invoices", "list_purchase_orders", "list_payments"]) {
      expect(rg(t), t).toEqual(["small", "large"]);
    }
    expect(rg("get_project")).toEqual([
      "estimate",
      "order",
      "delivery",
      "invoice",
      "receipt",
      "all",
    ]);
  });
});

describe("get の引数", () => {
  it.each(Object.entries(GET_TOOLS))("%s → %s は id / refresh / show_meta", (resource, tool) => {
    const expected = ["id", "refresh", "show_meta", "max_response_tokens"];
    if (resource === "projects") expected.push("response_group");
    expect(props(tool)).toEqual(expected.sort());
    expect(
      (z.toJSONSchema(byName.get(tool)?.inputSchema as z.ZodObject) as { required?: string[] })
        .required,
    ).toEqual(["id"]);
  });

  it("書類 4 種は refresh=full で取り直し、delta は何もしないことを description に書く", () => {
    for (const resource of DOCUMENTS) {
      const d = byName.get(GET_TOOLS[resource] as string)?.description ?? "";
      expect(d, resource).toContain("refresh=full");
      expect(d, resource).toContain("delta は何もしない");
    }
  });

  it("get_project は response_group 指定で常に API を直接呼び、refresh は無視することを書く", () => {
    const d = byName.get("get_project")?.description ?? "";
    expect(d).toContain("response_group");
    expect(d).toContain("refresh は無視");
  });
});

describe("引数の検証(いずれも argument_error。BOARD には通信しない)", () => {
  const cases: [string, string, Record<string, unknown>][] = [
    ["id が 0", "get_client", { id: 0 }],
    ["id が負", "get_client", { id: -1 }],
    ["id が小数", "get_delivery", { id: 1.5 }],
    ["id が文字列", "get_user", { id: "1" }],
    ["id がない", "get_group", {}],
    ["refresh が不正", "list_clients", { refresh: "daily" }],
    ["get の refresh が不正", "get_project", { id: 1, refresh: "all" }],
    ["response_group が不正(list)", "list_projects", { response_group: "huge" }],
    ["response_group が不正(clients は small / large)", "list_clients", { response_group: "all" }],
    [
      "response_group が不正(get_project は small を取れない)",
      "get_project",
      { id: 1, response_group: "small" },
    ],
    ["未知の引数(list)", "list_clients", { nope: 1 }],
    ["未知の引数(get)", "get_client", { id: 1, nope: 1 }],
    ["limit が負", "list_clients", { limit: -1 }],
    ["limit が小数", "list_clients", { limit: 1.5 }],
    ["show_meta が真偽値でない", "list_clients", { show_meta: "yes" }],
    ["tags が配列でない", "list_clients", { tags: "a" }],
    ["order_status_in の要素が整数でない", "list_projects", { order_status_in: [1.5] }],
    ["client_id が文字列", "list_contacts", { client_id: "1" }],
    ["max_response_tokens が負", "list_clients", { max_response_tokens: -1 }],
  ];
  it.each(cases)("%s", async (_label, tool, args) => {
    const r = await callApiTool(tool, args);
    expect(r.isError, r.text).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.calls).toHaveLength(0);
  });

  it("tools/list の inputSchema に未知の引数を許す指定(additionalProperties: true)がない", async () => {
    const res = await rpcTo(() => createMcpServer(), "tools/list");
    for (const t of res.result?.tools ?? []) {
      const s = t.inputSchema as { additionalProperties?: unknown } | undefined;
      expect(s?.additionalProperties === true, t.name).toBe(false);
    }
  });
});

describe("表と repository の整合", () => {
  it("cache-first のリソースは、cacheable な引数の対応先が M03 の cacheFirst.keys と一致する", () => {
    for (const def of API_RESOURCES.filter((d) => d.mode === "cache-first")) {
      const spec = SPECS[def.resource as keyof typeof SPECS] as unknown as {
        kind: string;
        cacheFirst: { keys: string[] };
      };
      expect(spec.kind, def.resource).toBe("cache-first");
      expect(
        def.fields
          .filter((f) => f.cacheable)
          .map((f) => f.opt)
          .sort(),
        def.resource,
      ).toEqual([...spec.cacheFirst.keys].sort());
    }
  });

  it("表のモードが repository の分類と一致し、list 18 + document 4 = get 22", () => {
    for (const def of API_RESOURCES) {
      const kind = (SPECS[def.resource as keyof typeof SPECS] as { kind: string }).kind;
      expect(def.mode, def.resource).toBe(kind);
    }
    expect(API_RESOURCES).toHaveLength(22);
    expect(API_RESOURCES.filter((d) => d.mode !== "document")).toHaveLength(18);
  });
});
