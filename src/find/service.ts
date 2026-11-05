/**
 * find 層の 11 の検索(Go 版 `find_*.go`)。repository(M03)からだけ読み、独自のキャッシュは持たない。
 *
 * - 主体の取得の失敗は致命的(そのまま伝える)。補助情報(顧客・案件・支店・担当者など)の失敗は null + 警告。
 * - 検索項目は AND で評価する。id を指定したら他の絞り込みは無視して直接取得する。
 * - Go は補助情報を errgroup で並列に取るが、Workers では BOARD クライアントが 1 つで直列化されるので逐次で書く。
 */
import type { ClientEntity, ProjectEntity } from "../board/resources";
import { filterProjectsByContractStatus } from "./contract-status";
import { filterByStatuses, filterProjectsByStatus, filterProjectsByStatuses } from "./filter";
import { resolveClientByName, resolveVendorByName } from "./resolver";
import type { FindSession } from "./session";
import {
  type ClientQuery,
  type ClientResult,
  type InvoiceResult,
  type PaymentResult,
  type ProjectQuery,
  type ProjectResult,
  type PurchaseOrderResult,
  type StatusedQuery,
  type UserQuery,
  type UserResult,
  type VendorQuery,
  type VendorResult,
  validateClientQuery,
  validateInvoiceQuery,
  validatePaymentQuery,
  validateProjectQuery,
  validatePurchaseOrderQuery,
  validateUserQuery,
  validateVendorQuery,
} from "./types";
import { clientUrl, documentUrl, projectUrl, vendorUrl } from "./url";

export * from "./documents";

/** url が空なら付けない(Go の `omitempty`)。 */
const withUrl = <T extends object>(result: T, url: string): T & { url?: string } =>
  url === "" ? result : { ...result, url };

// --- clients / vendors / users --------------------------------------------------------------

export async function findClients(s: FindSession, q: ClientQuery): Promise<ClientResult[]> {
  validateClientQuery(q);
  const clients: ClientEntity[] = q.id
    ? [await s.get("clients", q.id)]
    : q.name
      ? await s.search("clients", { nameCont: q.name })
      : [];
  const out: ClientResult[] = [];
  for (const c of s.applyLimit(clients, q.limit)) {
    const filter = { clientIdEq: c.id };
    const Branches = await s.softList("client_branches", filter, c.id);
    const Contacts = await s.softList("contacts", filter, c.id);
    out.push(withUrl({ Client: c, Branches, Contacts }, clientUrl(s.uiBaseUrl, c.id)));
  }
  return out;
}

export async function findVendors(s: FindSession, q: VendorQuery): Promise<VendorResult[]> {
  validateVendorQuery(q);
  const vendors = q.id
    ? [await s.get("vendors", q.id)]
    : q.name
      ? await s.search("vendors", { nameCont: q.name })
      : [];
  const out: VendorResult[] = [];
  for (const v of s.applyLimit(vendors, q.limit)) {
    // 仕入先の支店・担当者のフィルタキーは payee_id(client_id ではない)。
    const filter = { payeeIdEq: v.id };
    const Branches = await s.softList("vendor_branches", filter, v.id);
    const Contacts = await s.softList("vendor_contacts", filter, v.id);
    out.push(withUrl({ Vendor: v, Branches, Contacts }, vendorUrl(s.uiBaseUrl, v.id)));
  }
  return out;
}

export async function findUsers(s: FindSession, q: UserQuery): Promise<UserResult[]> {
  validateUserQuery(q);
  const users = q.id
    ? [await s.get("users", q.id)]
    : q.name
      ? await s.search("users", { nameCont: q.name })
      : [];
  return s.applyLimit(users, q.limit).map((User) => ({ User }));
}

// --- projects --------------------------------------------------------------------------------

export async function findProjects(s: FindSession, q: ProjectQuery): Promise<ProjectResult[]> {
  validateProjectQuery(q);
  const clientId = q.clientName ? await resolveClientByName(s, q.clientName) : q.clientId;

  let projects: ProjectEntity[];
  if (q.id) {
    projects = [await s.get("projects", q.id)];
  } else {
    projects = await s.search("projects", {
      ...(clientId ? { clientIdEq: clientId } : {}),
      ...(q.name ? { nameCont: q.name } : {}),
    });
    // status / statuses / contract_status は後段の絞り込み(id 指定のときは掛けない)。排他は validate 済み。
    if (q.contractStatus) projects = filterProjectsByContractStatus(projects, q.contractStatus);
    else if (q.statuses?.length) projects = filterProjectsByStatuses(projects, q.statuses);
    else if (q.status) projects = filterProjectsByStatus(projects, q.status);
  }

  const out: ProjectResult[] = [];
  for (const p of s.applyLimit(projects, q.limit)) {
    const cid = p.client?.id ?? 0;
    const Client = cid ? await s.getOptional("clients", cid) : null;
    out.push(withUrl({ Project: p, Client }, projectUrl(s.uiBaseUrl, p.id)));
  }
  return out;
}

// --- invoices / purchase_orders / payments ----------------------------------------------------

export async function findInvoices(s: FindSession, q: StatusedQuery): Promise<InvoiceResult[]> {
  validateInvoiceQuery(q);
  const clientId = q.partnerName ? await resolveClientByName(s, q.partnerName) : q.partnerId;
  let invoices = q.id
    ? [await s.get("invoices", q.id)]
    : await s.search("invoices", {
        ...(clientId ? { clientIdEq: clientId } : {}),
        ...(q.status ? { statusEq: q.status } : {}),
      });
  // statuses(複数)は API 側に対応する引数がないので後段で絞る。id 指定のときは掛けない。
  if (!q.id && q.statuses?.length)
    invoices = filterByStatuses(invoices, (i) => i.status, q.statuses);

  const out: InvoiceResult[] = [];
  for (const x of s.applyLimit(invoices, q.limit)) {
    const Client = x.client_id ? await s.getOptional("clients", x.client_id) : null;
    const Project = x.project_id ? await s.getOptional("projects", x.project_id) : null;
    out.push(withUrl({ Invoice: x, Project, Client }, documentUrl(s.uiBaseUrl, x.project_id)));
  }
  return out;
}

export async function findPurchaseOrders(
  s: FindSession,
  q: StatusedQuery,
): Promise<PurchaseOrderResult[]> {
  validatePurchaseOrderQuery(q);
  const vendorId = q.partnerName ? await resolveVendorByName(s, q.partnerName) : q.partnerId;
  let pos = q.id
    ? [await s.get("purchase_orders", q.id)]
    : await s.search("purchase_orders", {
        ...(vendorId ? { vendorIdEq: vendorId } : {}),
        ...(q.status ? { statusEq: q.status } : {}),
      });
  if (!q.id && q.statuses?.length) pos = filterByStatuses(pos, (p) => p.status, q.statuses);

  const out: PurchaseOrderResult[] = [];
  for (const x of s.applyLimit(pos, q.limit)) {
    const Vendor = x.vendor_id ? await s.getOptional("vendors", x.vendor_id) : null;
    const Project = x.project_id ? await s.getOptional("projects", x.project_id) : null;
    out.push(
      withUrl({ PurchaseOrder: x, Vendor, Project }, documentUrl(s.uiBaseUrl, x.project_id)),
    );
  }
  return out;
}

export async function findPayments(s: FindSession, q: StatusedQuery): Promise<PaymentResult[]> {
  validatePaymentQuery(q);
  const vendorId = q.partnerName ? await resolveVendorByName(s, q.partnerName) : q.partnerId;
  let payments = q.id
    ? [await s.get("payments", q.id)]
    : await s.search("payments", {
        ...(vendorId ? { vendorIdEq: vendorId } : {}),
        ...(q.status ? { statusEq: q.status } : {}),
      });
  if (!q.id && q.statuses?.length) {
    payments = filterByStatuses(payments, (p) => p.status, q.statuses);
  }

  const out: PaymentResult[] = [];
  for (const x of s.applyLimit(payments, q.limit)) {
    const Vendor = x.vendor_id ? await s.getOptional("vendors", x.vendor_id) : null;
    // 支払は案件 ID を持たないので Project は常に null、URL も付けない(Go と同じ)。
    out.push({ Payment: x, Vendor, Project: null });
  }
  return out;
}
