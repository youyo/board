/**
 * find 層のツール 11 個(M05)。名前・引数・description は Go 版 `internal/mcpserver/tools.go` と同じ。
 * Go の MCP にあった `refresh` 引数と、応答の `cache` 配列は維持する。
 *
 * 応答: `{ items, cache, warnings?, limited?, total_before_limit? }`
 *  - items: Go の find の結果(キーは Go の出力と同じ。中のエンティティは BOARD のキーのまま)。
 *  - cache: 読んだリソースの鮮度(`cached_at` / `full_refreshed_at` / `refresh_state`)。
 *  - warnings: 致命的でない問題(補助情報の取得失敗・案件 ID を解決できなかった・一覧の打ち切り)。コードと対象 ID だけで、例外の文面は載せない。
 *  - limited / total_before_limit: `limit` で件数を切ったときだけ。
 * ツール呼び出しごとに `CallContext` は 1 つ(締め切りは全体で共有する)。
 */
import { z } from "zod";
import { ArgumentError } from "../../errors";
import {
  findClients,
  findDeliveries,
  findEstimates,
  findInvoices,
  findOrders,
  findPayments,
  findProjects,
  findPurchaseOrders,
  findReceipts,
  findUsers,
  findVendors,
} from "../../find/service";
import { FindSession } from "../../find/session";
import type { DocumentQuery } from "../../find/types";
import type { RefreshChoice } from "../../repository";
import {
  idOrUndefined,
  optEnum,
  optId,
  optInt,
  optNonNegativeInt,
  optStr,
  optStrArray,
  REFRESH_CHOICES,
} from "../args";
import { callContextOf } from "../call-context";
import { defineTool, type ToolContext, type ToolDefinition } from "../registry";

// --- description(Go の helper と同じ文面) -----------------------------------------------------

const disambiguateNameDesc = (target: string, entity: string) =>
  `Resolve ${target} name (substring match) to filter ${entity}. Returns an ambiguity error with up to 5 candidates if multiple ${target}s match; use id to disambiguate.`;

const fanoutNameDesc = (target: string, narrowProp: string) =>
  `Resolve ${target} name (substring match) and fanout-search across all matching ${target}s (no disambiguation). Use ${narrowProp} to narrow to a single document. More than 5 matches is rejected (narrow the query).`;

const notYetSupportedDesc = (field: string, scope: string) =>
  `(NOT YET SUPPORTED) Will filter ${scope} by ${field} in a future release. Currently returns an error.`;

const REFRESH_DESC =
  'Trigger a refresh before reading cache. "none" (default) reads cache as-is. "delta" fetches incremental changes since last sync. "full" re-fetches all entries and removes stale ones (use periodically; a large resource may need several calls, see refresh_state=partial in cache). Concurrent refreshes return refresh_in_progress; clients should wait and retry.';

// Go の説明は "default: 50, max: 100" だが、Go の実装は 0・未指定を無制限として扱い上限も強制していない。事実に合わせて書く。
const LIMIT_DESC =
  "Max results to return (0 or omitted = unlimited). When the limit cuts the results, the response has limited: true and total_before_limit (the count before cutting). Set a limit when a name may match many entities.";

const RESPONSE_DESC =
  " Response: {items, cache, warnings?, limited?, total_before_limit?}. cache = freshness of the cached resources that were read (cached_at, full_refreshed_at, refresh_state); warnings = non-fatal problems (enrichment that failed and is null, a project_id that could not be resolved, a truncated list).";

// --- 共通 ---------------------------------------------------------------------------------

type Common = { refresh?: RefreshChoice; limit?: number };

const commonShape = () => ({
  refresh: optEnum(REFRESH_CHOICES, REFRESH_DESC),
  limit: optNonNegativeInt(LIMIT_DESC),
});

function newSession(ctx: ToolContext, refresh: RefreshChoice | undefined): FindSession {
  return new FindSession({
    repos: {
      ...ctx.repositories(),
      // 書類の検索だけが使う。最初に使われたときに取り出す。
      documentIndex: {
        lookup: (kind, id, c) => ctx.documentIndex().lookup(kind, id, c),
        record: (kind, projects) => ctx.documentIndex().record(kind, projects),
      },
    },
    ctx: callContextOf(ctx),
    refresh,
    uiBaseUrl: ctx.uiBaseUrl,
  });
}

function output(s: FindSession, items: unknown[]) {
  const warnings = s.warnings;
  return {
    items,
    cache: s.cache,
    ...(warnings.length > 0 ? { warnings } : {}),
    ...(s.totalBeforeLimit !== undefined
      ? { limited: true, total_before_limit: s.totalBeforeLimit }
      : {}),
  };
}

const idDesc = (what: string) => `${what} ID for direct lookup (highest priority).`;

/** 値が空でない文字列か(Go の getStringArg が '' を『指定なし』とするのに合わせる)。 */
const given = (v: unknown): v is string => typeof v === "string" && v !== "";

// --- ツール -------------------------------------------------------------------------------

function simpleTool(
  kind: "client" | "vendor" | "user",
  plural: string,
  returns: string,
  run: (s: FindSession, q: { id?: number; name?: string; limit?: number }) => Promise<unknown[]>,
): ToolDefinition {
  return defineTool({
    name: `find_${plural}`,
    title: `${kind === "client" ? "顧客" : kind === "vendor" ? "仕入先" : "ユーザー"}検索`,
    description: `Search BOARD ${plural} by ID or name. Returns ${returns}. Priority: id > name.${RESPONSE_DESC}`,
    category: "ReadOnly",
    inputSchema: z.strictObject({
      id: optId(
        `${kind[0]?.toUpperCase()}${kind.slice(1)} ID for direct lookup (highest priority; ignores name).`,
      ),
      name: optStr(`Substring match on ${kind} name.`),
      ...commonShape(),
    }),
    handler: async (args, ctx) => {
      const a = args as Common & { id?: number; name?: string };
      const s = newSession(ctx, a.refresh);
      return output(s, await run(s, { id: idOrUndefined(a.id), name: a.name, limit: a.limit }));
    },
  });
}

const findClientsTool = simpleTool(
  "client",
  "clients",
  "client entities with branches and contacts",
  findClients,
);
const findVendorsTool = simpleTool(
  "vendor",
  "vendors",
  "vendor entities with branches and contacts",
  findVendors,
);
const findUsersTool = simpleTool("user", "users", "user entities", findUsers);

const findProjectsTool = defineTool({
  name: "find_projects",
  title: "案件検索",
  description: `Search BOARD projects by ID, client name, or project name. Fields combine with AND (ID overrides others). Returns project entities with enriched client info. client_name resolves with ambiguity error on multiple matches. status/statuses/contract_status filtering requires narrowing (must combine with id/client_name/name). status, statuses, contract_status are mutually exclusive.${RESPONSE_DESC}`,
  category: "ReadOnly",
  inputSchema: z.strictObject({
    id: optId("Project ID for direct lookup (highest priority; ignores other filters)."),
    client_name: optStr(disambiguateNameDesc("client", "project")),
    name: optStr("Substring match on project name."),
    status: optStr(
      "Filter by project status (e.g. 受注, 完了). MUST be combined with id/client_name/name — status-only query is rejected (API delegation not possible, narrowing required). Mutually exclusive with statuses / contract_status.",
    ),
    statuses: optStrArray(
      "Filter by multiple project statuses (OR). Mutually exclusive with status / contract_status. Same narrowing rules apply. Max 10 items.",
    ),
    contract_status: optStr(
      "Contract status alias. Valid values: active (in-progress: 未着手/着手中/納品済), ended (検収済), prospect (見積中*), all. Mutually exclusive with status / statuses. Same narrowing rules apply.",
    ),
    ...commonShape(),
  }),
  handler: async (args, ctx) => {
    const a = args as Common & {
      id?: number;
      client_name?: string;
      name?: string;
      status?: string;
      statuses?: string[];
      contract_status?: string;
    };
    const s = newSession(ctx, a.refresh);
    return output(
      s,
      await findProjects(s, {
        id: idOrUndefined(a.id),
        clientName: a.client_name,
        name: a.name,
        status: a.status,
        statuses: a.statuses,
        contractStatus: a.contract_status,
        limit: a.limit,
      }),
    );
  },
});

function documentTool(
  kind: "estimate" | "order" | "delivery" | "receipt",
  plural: string,
  idWhat: string,
  projectIdDesc: string,
  run: (s: FindSession, q: DocumentQuery) => Promise<unknown[]>,
): ToolDefinition {
  return defineTool({
    name: `find_${plural}`,
    title: `${{ estimate: "見積書", order: "注文書", delivery: "納品書", receipt: "領収書" }[kind]}検索`,
    description: `Search BOARD ${kind} documents by ID, project ID, client name, or project name. Returns ${kind} entities (no Status field on entity, hence no status filter). client_name/project_name perform fanout search across all matching entities (no disambiguation).${RESPONSE_DESC} With client_name and limit, the search stops once limit candidates are found, so total_before_limit is the number of candidates found so far (not exact). More than 200 candidate documents are rejected (argument_error): narrow with project_id or set limit. Each item has ProjectID/ClientID resolved from the document ID (0 with a project_unresolved warning if it could not be resolved in time; retry to continue).`,
    category: "ReadOnly",
    inputSchema: z.strictObject({
      id: optId(`${idWhat} for direct lookup (highest priority).`),
      project_id: optId(projectIdDesc),
      client_name: optStr(fanoutNameDesc("client", "project_id")),
      project_name: optStr(fanoutNameDesc("project", "project_id")),
      ...commonShape(),
    }),
    handler: async (args, ctx) => {
      const a = args as Common & {
        id?: number;
        project_id?: number;
        client_name?: string;
        project_name?: string;
      };
      const s = newSession(ctx, a.refresh);
      return output(
        s,
        await run(s, {
          id: idOrUndefined(a.id),
          projectId: idOrUndefined(a.project_id),
          clientName: a.client_name,
          projectName: a.project_name,
          limit: a.limit,
        }),
      );
    },
  });
}

const findEstimatesTool = documentTool(
  "estimate",
  "estimates",
  "Estimate document ID",
  "Project ID to find its estimate (use to narrow to a single document).",
  findEstimates,
);
const findOrdersTool = documentTool(
  "order",
  "orders",
  "Order document ID",
  "Project ID to find its order (use to narrow to a single document).",
  findOrders,
);
const findDeliveriesTool = documentTool(
  "delivery",
  "deliveries",
  "Delivery document ID",
  "Project ID to find its deliveries (use to narrow to a single project).",
  findDeliveries,
);
const findReceiptsTool = documentTool(
  "receipt",
  "receipts",
  "Receipt document ID",
  "Project ID to find its receipts (use to narrow to a single project).",
  findReceipts,
);

const findInvoicesTool = defineTool({
  name: "find_invoices",
  title: "請求書検索",
  description: `Search BOARD invoices by ID, client name, or status. Fields combine with AND (ID overrides others). Returns invoice entities. client_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported.${RESPONSE_DESC}`,
  category: "ReadOnly",
  inputSchema: z.strictObject({
    id: optId(idDesc("Invoice")),
    client_name: optStr(disambiguateNameDesc("client", "invoice")),
    project_name: optStr(notYetSupportedDesc("project name", "invoices")),
    status: optStr(
      "Filter by invoice status (single value; API delegated, no narrowing required).",
    ),
    ...commonShape(),
  }),
  handler: async (args, ctx) => {
    const a = args as Common & {
      id?: number;
      client_name?: string;
      project_name?: string;
      status?: string;
    };
    if (given(a.project_name)) {
      throw new ArgumentError(
        "project_name is not yet supported for invoices (tracked for future enhancement)",
      );
    }
    const s = newSession(ctx, a.refresh);
    return output(
      s,
      await findInvoices(s, {
        id: idOrUndefined(a.id),
        partnerName: a.client_name,
        status: a.status,
        limit: a.limit,
      }),
    );
  },
});

const findPurchaseOrdersTool = defineTool({
  name: "find_purchase_orders",
  title: "発注書検索",
  description: `Search BOARD purchase orders by ID, vendor name, or status. Fields combine with AND (ID overrides others). Returns purchase order entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). project_name is not yet supported.${RESPONSE_DESC}`,
  category: "ReadOnly",
  inputSchema: z.strictObject({
    id: optId(idDesc("Purchase order")),
    vendor_name: optStr(disambiguateNameDesc("vendor", "purchase order")),
    project_name: optStr(notYetSupportedDesc("project name", "purchase orders")),
    status: optStr(
      "Filter by purchase order status (single value; API delegated, no narrowing required).",
    ),
    ...commonShape(),
  }),
  handler: async (args, ctx) => {
    const a = args as Common & {
      id?: number;
      vendor_name?: string;
      project_name?: string;
      status?: string;
    };
    if (given(a.project_name)) {
      throw new ArgumentError(
        "project_name is not yet supported for purchase orders (tracked for future enhancement)",
      );
    }
    const s = newSession(ctx, a.refresh);
    return output(
      s,
      await findPurchaseOrders(s, {
        id: idOrUndefined(a.id),
        partnerName: a.vendor_name,
        status: a.status,
        limit: a.limit,
      }),
    );
  },
});

const findPaymentsTool = defineTool({
  name: "find_payments",
  title: "支払検索",
  description: `Search BOARD payments by ID, vendor name, or status. Fields combine with AND (ID overrides others). Returns payment entities. vendor_name resolves with ambiguity error on multiple matches. status accepts a single status (API delegated; no narrowing required). purchase_order_id is not yet supported.${RESPONSE_DESC}`,
  category: "ReadOnly",
  inputSchema: z.strictObject({
    id: optId(idDesc("Payment")),
    vendor_name: optStr(disambiguateNameDesc("vendor", "payment")),
    purchase_order_id: optInt(notYetSupportedDesc("purchase order ID", "payments")),
    status: optStr(
      "Filter by payment status (single value; API delegated, no narrowing required).",
    ),
    ...commonShape(),
  }),
  handler: async (args, ctx) => {
    const a = args as Common & {
      id?: number;
      vendor_name?: string;
      purchase_order_id?: number;
      status?: string;
    };
    if (a.purchase_order_id) {
      throw new ArgumentError(
        "purchase_order_id is not yet supported (tracked for future enhancement)",
      );
    }
    const s = newSession(ctx, a.refresh);
    return output(
      s,
      await findPayments(s, {
        id: idOrUndefined(a.id),
        partnerName: a.vendor_name,
        status: a.status,
        limit: a.limit,
      }),
    );
  },
});

/** Go の MCP と同じ並び(`RegisterTools`)。 */
export const FIND_TOOLS: readonly ToolDefinition[] = [
  findClientsTool,
  findVendorsTool,
  findUsersTool,
  findProjectsTool,
  findEstimatesTool,
  findInvoicesTool,
  findOrdersTool,
  findDeliveriesTool,
  findReceiptsTool,
  findPurchaseOrdersTool,
  findPaymentsTool,
];
