/**
 * find 層の 11 の検索(Go 版 `find_*.go`)。repository(M03)からだけ読み、独自のキャッシュは持たない。
 *
 * - 主体の取得の失敗は致命的(そのまま伝える)。補助情報(顧客・案件・支店・担当者など)の失敗は null + 警告。
 * - 検索項目は AND で評価する。id を指定したら他の絞り込みは無視して直接取得する。
 * - Go は補助情報を errgroup で並列に取るが、Workers では BOARD クライアントが 1 つで直列化されるので逐次で書く。
 */
import type { ClientEntity, ProjectEntity } from "../board/resources";
import { filterProjectsByContractStatus } from "./contract-status";
import { filterProjectsByStatus, filterProjectsByStatuses } from "./filter";
import { resolveClientByName, resolveProjectByName, resolveVendorByName } from "./resolver";
import type { FindSession } from "./session";
import { invoiceStatusCodes, paymentStatusCodes, purchaseOrderStatusFilter } from "./status";
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

/** status(単独)と statuses(複数)は排他(validate 済み)。どちらも名称で受ける。 */
const statusNames = (q: StatusedQuery): string[] =>
  q.statuses?.length ? q.statuses : q.status ? [q.status] : [];

export async function findInvoices(s: FindSession, q: StatusedQuery): Promise<InvoiceResult[]> {
  validateInvoiceQuery(q);
  // ステータス名はコードに直して API / キャッシュの絞り込みに渡す。id 指定のときは掛けない(通信の前に検査する)。
  const statusCodes = q.id ? [] : invoiceStatusCodes(statusNames(q));
  const clientId = q.id
    ? undefined
    : q.partnerName
      ? await resolveClientByName(s, q.partnerName)
      : q.partnerId;
  // 案件の指定(project_id / project_name)は案件を 1 件に解決し、その案件番号で API / キャッシュを絞る。
  // 案件番号が無い案件は番号では絞れないので、どちらの場合も後段で project_id を突き合わせる。
  let project: { id: number; project_no: number | null } | undefined;
  if (!q.id) {
    if (q.projectName) project = await resolveProjectByName(s, q.projectName, clientId);
    else if (q.projectId) project = await s.get("projects", q.projectId);
  }
  let invoices = q.id
    ? [await s.get("invoices", q.id)]
    : await s.search("invoices", {
        ...(clientId ? { projectClientIdEq: clientId } : {}),
        ...(statusCodes.length ? { invoiceStatusIn: statusCodes } : {}),
        ...(project?.project_no ? { projectProjectNoEq: project.project_no } : {}),
      });
  if (project) {
    const pid = project.id;
    invoices = invoices.filter((x) => x.project_id === pid);
  }

  const out: InvoiceResult[] = [];
  for (const x of s.applyLimit(invoices, q.limit)) {
    const cid = x.client?.id ?? 0;
    const Client = cid ? await s.getOptional("clients", cid) : null;
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
  const { expenditureStatusIn, deliveryStatusIn } = q.id
    ? { expenditureStatusIn: [], deliveryStatusIn: [] }
    : purchaseOrderStatusFilter(statusNames(q));
  // 発注ステータスと進捗状況の名称が混ざったら OR にしたいが、API の引数どうしは AND になる。
  // その場合は API には絞り込みなしで取り(相手先で絞る)、後段で OR にする。
  const mixed = expenditureStatusIn.length > 0 && deliveryStatusIn.length > 0;
  const payeeId = q.partnerName ? await resolveVendorByName(s, q.partnerName) : q.partnerId;
  let pos = q.id
    ? [await s.get("purchase_orders", q.id)]
    : await s.search("purchase_orders", {
        ...(payeeId ? { payeeIdEq: payeeId } : {}),
        ...(!mixed && expenditureStatusIn.length ? { expenditureStatusIn } : {}),
        ...(!mixed && deliveryStatusIn.length ? { deliveryStatusIn } : {}),
      });
  if (mixed) {
    pos = pos.filter(
      (p) =>
        expenditureStatusIn.includes(p.expenditure_status) ||
        deliveryStatusIn.includes(p.delivery_status),
    );
  }

  const out: PurchaseOrderResult[] = [];
  for (const x of s.applyLimit(pos, q.limit)) {
    const vid = x.payee?.id ?? 0;
    const pid = x.project?.id ?? 0;
    const Vendor = vid ? await s.getOptional("vendors", vid) : null;
    const Project = pid ? await s.getOptional("projects", pid) : null;
    out.push(withUrl({ PurchaseOrder: x, Vendor, Project }, documentUrl(s.uiBaseUrl, pid)));
  }
  return out;
}

export async function findPayments(s: FindSession, q: StatusedQuery): Promise<PaymentResult[]> {
  validatePaymentQuery(q);
  const statusCodes = q.id ? [] : paymentStatusCodes(statusNames(q));
  const payeeId = q.partnerName ? await resolveVendorByName(s, q.partnerName) : q.partnerId;
  const payments = q.id
    ? [await s.get("payments", q.id)]
    : await s.search("payments", {
        ...(payeeId ? { expenditurePayeeIdEq: payeeId } : {}),
        ...(statusCodes.length ? { paymentStatusIn: statusCodes } : {}),
      });

  const out: PaymentResult[] = [];
  for (const x of s.applyLimit(payments, q.limit)) {
    const vid = x.payee?.id ?? 0;
    const Vendor = vid ? await s.getOptional("vendors", vid) : null;
    // 支払は案件を持たない(project は入っても Project は付けない)ので Project は常に null、URL も付けない(Go と同じ)。
    out.push({ Payment: x, Vendor, Project: null });
  }
  return out;
}
