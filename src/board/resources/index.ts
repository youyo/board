/** BOARD API の 22 リソース。Go 版 `internal/boardapi` と `internal/cli/api_*.go` の実態に合わせる。 */

export * from "./accounting_types";
export * from "./client_branches";
export * from "./clients";
export * from "./common";
export * from "./contacts";
export * from "./deliveries";
export * from "./document_send_channels";
export * from "./estimates";
export * from "./groups";
export * from "./invoices";
export * from "./orders";
export * from "./payment_terms";
export * from "./payments";
export * from "./project_costs";
export * from "./project_types";
export * from "./projects";
export * from "./purchase_orders";
export * from "./purchase_types";
export * from "./receipts";
export * from "./refs";
export * from "./users";
export * from "./vendor_branches";
export * from "./vendor_contacts";
export * from "./vendors";

export interface ResourceInfo {
  /** Go 版 CLI のリソース名(`board api <name>`) */
  name: string;
  /** BOARD API のパス(get は末尾に `/{id}`) */
  path: string;
  list: boolean;
  get: boolean;
}

const r = (name: string, path: string, list = true): ResourceInfo => ({
  name,
  path,
  list,
  get: true,
});

export const RESOURCES: readonly ResourceInfo[] = [
  r("clients", "/v1/clients"),
  r("client_branches", "/v1/client_branches"),
  r("contacts", "/v1/contacts"),
  r("projects", "/v1/projects"),
  r("project_costs", "/v1/project_costs"),
  r("estimates", "/v1/documents/estimates", false),
  r("invoices", "/v1/invoices"),
  r("orders", "/v1/documents/orders", false),
  r("deliveries", "/v1/documents/deliveries", false),
  r("receipts", "/v1/documents/receipts", false),
  r("vendors", "/v1/payees"),
  r("vendor_branches", "/v1/payee_branches"),
  r("vendor_contacts", "/v1/payee_contacts"),
  r("purchase_orders", "/v1/expenditures"),
  r("payments", "/v1/expenditure_payments"),
  r("users", "/v1/users"),
  r("groups", "/v1/groups"),
  r("payment_terms", "/v1/payment_terms"),
  r("project_types", "/v1/project_types"),
  r("purchase_types", "/v1/expenditure_types"),
  r("accounting_types", "/v1/accounting_types"),
  r("document_send_channels", "/v1/document_send_channels"),
];
