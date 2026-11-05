/**
 * find 層のツール 11 個の定義(Go 版 `internal/mcpserver/tools.go` と名前・引数・description を揃える)と、引数の検証。
 */
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { BASE, callApiTool } from "../helpers/api-tool";
import { jsonResponse } from "../helpers/mock-fetch";

const FIND_TOOLS: Record<string, string[]> = {
  find_clients: ["id", "name"],
  find_vendors: ["id", "name"],
  find_users: ["id", "name"],
  find_projects: ["id", "client_name", "name", "status", "statuses", "contract_status"],
  find_estimates: ["id", "project_id", "client_name", "project_name"],
  find_invoices: ["id", "client_name", "project_name", "status"],
  find_orders: ["id", "project_id", "client_name", "project_name"],
  find_deliveries: ["id", "project_id", "client_name", "project_name"],
  find_receipts: ["id", "project_id", "client_name", "project_name"],
  find_purchase_orders: ["id", "vendor_name", "project_name", "status"],
  find_payments: ["id", "vendor_name", "purchase_order_id", "status"],
};
const COMMON = ["refresh", "limit", "max_response_tokens"];

const byName = new Map(ALL_TOOLS.map((t) => [t.name, t]));
const schemaOf = (name: string) =>
  z.toJSONSchema(byName.get(name)?.inputSchema as z.ZodObject) as {
    properties: Record<string, { description?: string; enum?: string[]; type?: string }>;
    additionalProperties?: boolean;
  };

describe("find 層のツール 11 個", () => {
  it("名前は Go の MCP と同じ 11 個で、接頭辞がなく、すべて ReadOnly", () => {
    const found = ALL_TOOLS.map((t) => t.name).filter((n) => n.startsWith("find_"));
    expect(found.sort()).toEqual(Object.keys(FIND_TOOLS).sort());
    for (const name of Object.keys(FIND_TOOLS)) {
      expect(byName.get(name)?.category).toBe("ReadOnly");
    }
  });

  it.each(Object.entries(FIND_TOOLS))(
    "%s の引数は Go と同じ(+ refresh / limit / max_response_tokens)",
    (name, args) => {
      expect(Object.keys(schemaOf(name).properties).sort()).toEqual([...args, ...COMMON].sort());
      expect(schemaOf(name).additionalProperties).toBe(false);
    },
  );

  it("書類 4 種に status の引数はない(構造的に存在しない)", () => {
    for (const n of ["find_estimates", "find_orders", "find_deliveries", "find_receipts"]) {
      expect("status" in schemaOf(n).properties).toBe(false);
    }
  });

  it("refresh は none / delta / full の enum、limit は説明に limited / total_before_limit を書く", () => {
    for (const name of Object.keys(FIND_TOOLS)) {
      const p = schemaOf(name).properties;
      expect(p.refresh?.enum).toEqual(["none", "delta", "full"]);
      expect(p.limit?.description, name).toContain("limited");
      expect(p.limit?.description, name).toContain("total_before_limit");
    }
  });

  it("description: disambiguate 系と fanout 系の違い・narrowing・未対応フラグを Go と同じ言葉で書く", () => {
    const d = (n: string) => byName.get(n)?.description ?? "";
    expect(d("find_clients")).toContain("Priority: id > name.");
    for (const n of ["find_projects", "find_invoices", "find_purchase_orders", "find_payments"]) {
      expect(d(n), n).toContain("Fields combine with AND (ID overrides others)");
      expect(d(n), n).toContain("ambiguity error");
    }
    expect(d("find_projects")).toContain("requires narrowing");
    expect(d("find_projects")).toContain("mutually exclusive");
    for (const n of ["find_estimates", "find_orders", "find_deliveries", "find_receipts"]) {
      expect(d(n), n).toContain("fanout search across all matching entities (no disambiguation)");
      expect(d(n), n).toContain("no status filter");
    }
    const props = (n: string, k: string) => schemaOf(n).properties[k]?.description ?? "";
    expect(props("find_invoices", "project_name")).toContain("(NOT YET SUPPORTED)");
    expect(props("find_purchase_orders", "project_name")).toContain("(NOT YET SUPPORTED)");
    expect(props("find_payments", "purchase_order_id")).toContain("(NOT YET SUPPORTED)");
    expect(props("find_estimates", "client_name")).toContain("Use project_id to narrow");
    expect(props("find_projects", "client_name")).toContain("up to 5 candidates");
    // 応答の形(Workers 版で足した warnings / limited)を書く。
    for (const n of Object.keys(FIND_TOOLS)) {
      expect(d(n), n).toContain("warnings");
      expect(d(n), n).toContain("cache");
    }
  });
});

describe("引数の検証(BOARD に通信する前に拒否する)", () => {
  const cases: [string, Record<string, unknown>, string][] = [
    ["find_clients", {}, "at least one field required"],
    // id: 0 は『指定なし』(Go の getIntArg と同じ)なので、ほかの指定がなければ『少なくとも 1 つ必要』になる。
    ["find_clients", { id: 0 }, "at least one field required"],
    ["find_clients", { id: -1 }, "id"],
    ["find_estimates", { id: 0, project_id: 0 }, "at least one field required"],
    ["find_estimates", { project_id: -3 }, "project_id"],
    ["find_clients", { id: 1.5 }, "id"],
    ["find_clients", { id: 1, limit: -1 }, "limit"],
    ["find_clients", { id: 1, bogus: true }, "bogus"],
    ["find_projects", { status: "受注" }, "requires at least one of ID, Name, or Client"],
    ["find_projects", { name: "x", status: "a", statuses: ["b"] }, "mutually exclusive"],
    ["find_projects", { name: "x", contract_status: "nope" }, "unknown contract_status"],
    ["find_estimates", { status: "x" }, "status"],
    ["find_estimates", {}, "at least one field required"],
    ["find_invoices", { project_name: "x" }, "project_name is not yet supported for invoices"],
    [
      "find_purchase_orders",
      { project_name: "x" },
      "project_name is not yet supported for purchase orders",
    ],
    ["find_payments", { purchase_order_id: 5 }, "purchase_order_id is not yet supported"],
  ];
  it.each(cases)("%s %j → argument_error", async (tool, args, message) => {
    const r = await callApiTool(tool, args, {});
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain(message);
    expect(r.calls).toHaveLength(0);
  });
});

describe("id: 0 は『指定なし』として扱う(Go の getIntArg と同じ)", () => {
  it("find_clients {id: 0, name} は name で検索する", async () => {
    const r = await callApiTool(
      "find_clients",
      { id: 0, name: "株式" },
      {
        [`GET ${BASE}/v1/clients`]: () => jsonResponse([{ id: 1, name: "株式会社A" }]),
        [`GET ${BASE}/v1/client_branches`]: () => jsonResponse([]),
        [`GET ${BASE}/v1/contacts`]: () => jsonResponse([]),
      },
    );
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0].Client.id).toBe(1);
    expect(r.calls.some((c) => new URL(c.url).pathname === "/v1/clients/0")).toBe(false);
  });

  it("find_estimates {id: 0, project_id: 5} は project_id で検索する", async () => {
    const r = await callApiTool(
      "find_estimates",
      { id: 0, project_id: 5 },
      {
        [`GET ${BASE}/v1/projects/5`]: () => jsonResponse({ id: 5, estimate: { id: 50 } }),
        [`GET ${BASE}/v1/documents/estimates/50`]: () => jsonResponse({ id: 50 }),
        [`GET ${BASE}/v1/clients/1`]: () => jsonResponse({ id: 1 }),
      },
    );
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items[0].ProjectID).toBe(5);
  });
});
