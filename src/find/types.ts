/**
 * find 層のクエリと結果(Go 版 `types.go`)。
 *
 * 結果のキーは Go の `json.Marshal` の出力と同じにする: Result 構造体には `url` 以外に JSON タグがなく、
 * Go の MCP はフィールド名(`Client` `Branches` `ProjectID` ...)のまま返していた。中のエンティティは BOARD のキー(snake_case)のまま。
 */
import type {
  ClientBranchEntity,
  ClientEntity,
  ContactEntity,
  DeliveryEntity,
  EstimateEntity,
  InvoiceEntity,
  OrderEntity,
  PaymentEntity,
  ProjectEntity,
  PurchaseOrderEntity,
  ReceiptEntity,
  UserEntity,
  VendorBranchEntity,
  VendorContactEntity,
  VendorEntity,
} from "../board/resources";
import { ArgumentError } from "../errors";
import { expandContractStatus } from "./contract-status";

// --- クエリ -------------------------------------------------------------------------------

/** 0・未指定は『指定なし』(Go のゼロ値と同じ)。 */
export interface CommonQuery {
  /** 返却件数の上限。0 は無制限。負数は拒否。 */
  limit?: number;
}

export interface ClientQuery extends CommonQuery {
  id?: number;
  name?: string;
}
export type VendorQuery = ClientQuery;
export type UserQuery = ClientQuery;

export interface ProjectQuery extends CommonQuery {
  id?: number;
  name?: string;
  /** 解決済みの顧客 ID。 */
  clientId?: number;
  /** 名前から ID を解決する前の指定(narrowing の判定にだけ使う)。 */
  clientName?: string;
  status?: string;
  statuses?: string[];
  contractStatus?: string;
}

export interface DocumentQuery extends CommonQuery {
  id?: number;
  projectId?: number;
  clientName?: string;
  projectName?: string;
}

/** 請求書・発注書・支払。相手先 ID は請求書=顧客、発注書・支払=仕入先。 */
export interface StatusedQuery extends CommonQuery {
  id?: number;
  partnerId?: number;
  /** 名前から ID を解決する前の指定(narrowing の判定にだけ使う)。 */
  partnerName?: string;
  status?: string;
  statuses?: string[];
  /** 請求書だけ: 案件 ID(project_name と排他)。 */
  projectId?: number;
  /** 請求書だけ: 案件名(部分一致で 1 件に解決する。project_id と排他)。 */
  projectName?: string;
}

// --- 検証(Go の validate) -----------------------------------------------------------------

const set = (v: unknown) => v !== undefined && v !== null && v !== "" && v !== 0;

function validateCommon(q: CommonQuery): void {
  if ((q.limit ?? 0) < 0) throw new ArgumentError("limit must be >= 0");
}

function validateStatusGroup(status?: string, statuses?: string[], contractStatus?: string): void {
  const n = [set(status), (statuses?.length ?? 0) > 0, set(contractStatus)].filter(Boolean).length;
  if (n > 1) {
    throw new ArgumentError(
      "invalid query: status / statuses / contract_status are mutually exclusive",
    );
  }
  if ((statuses?.length ?? 0) > 10) throw new ArgumentError("at most 10 statuses allowed");
}

const atLeastOne = () => new ArgumentError("at least one field required");

export function validateClientQuery(q: ClientQuery): void {
  validateCommon(q);
  if (!set(q.id) && !set(q.name)) throw atLeastOne();
}
export const validateVendorQuery = validateClientQuery;
export const validateUserQuery = validateClientQuery;

export function validateProjectQuery(q: ProjectQuery): void {
  validateCommon(q);
  validateStatusGroup(q.status, q.statuses, q.contractStatus);
  // 未知の contract_status は BOARD に通信する前に拒否する(Go は検索のあとに判定していた)。
  expandContractStatus(q.contractStatus ?? "");
  const hasStatus = set(q.status) || (q.statuses?.length ?? 0) > 0 || set(q.contractStatus);
  const hasNarrow = set(q.id) || set(q.name) || set(q.clientId) || set(q.clientName);
  if (!hasNarrow && !hasStatus) throw atLeastOne();
  // status 系は API に委譲できず全件走査になるので、絞り込みとの併用を必須にする。
  if (hasStatus && !hasNarrow) {
    throw new ArgumentError(
      "Status/Statuses/ContractStatus requires at least one of ID, Name, or Client (client_name) to narrow results",
    );
  }
}

export function validateDocumentQuery(q: DocumentQuery): void {
  validateCommon(q);
  if (!set(q.id) && !set(q.projectId) && !set(q.clientName) && !set(q.projectName)) {
    throw atLeastOne();
  }
}

/** 請求書・発注書・支払の共通の検証。`narrow` はエラー文に使う相手先の名前(ID / ClientID / VendorID)。 */
export function validateStatusedQuery(q: StatusedQuery, narrow: string, extraNarrow = false): void {
  validateCommon(q);
  validateStatusGroup(q.status, q.statuses);
  if (
    !set(q.id) &&
    !set(q.partnerId) &&
    !set(q.partnerName) &&
    !extraNarrow &&
    !set(q.status) &&
    !q.statuses?.length
  ) {
    throw atLeastOne();
  }
  // statuses(複数)だけでは API に委譲できず全件走査になる。単独の status は status_eq で委譲できる。
  if (
    (q.statuses?.length ?? 0) > 0 &&
    !set(q.id) &&
    !set(q.partnerId) &&
    !set(q.partnerName) &&
    !extraNarrow
  ) {
    throw new ArgumentError(
      `invalid query: Statuses requires one of ID or ${narrow} to narrow results`,
    );
  }
}
export function validateInvoiceQuery(q: StatusedQuery): void {
  if (set(q.projectId) && set(q.projectName)) {
    throw new ArgumentError("invalid query: project_id / project_name are mutually exclusive");
  }
  if ((q.projectId ?? 0) < 0) throw new ArgumentError("project_id must be >= 0");
  // 案件の指定は絞り込みとして数える(案件だけ・案件 + statuses でも検索できる)。
  validateStatusedQuery(q, "ClientID", set(q.projectId) || set(q.projectName));
}
export const validatePurchaseOrderQuery = (q: StatusedQuery) =>
  validateStatusedQuery(q, "VendorID");
export const validatePaymentQuery = (q: StatusedQuery) => validateStatusedQuery(q, "VendorID");

// --- 結果(キーは Go の出力と同じ) --------------------------------------------------------

/** `url` は空なら出さない(Go の `omitempty`)。 */
export interface WithUrl {
  url?: string;
}

export interface ClientResult extends WithUrl {
  Client: ClientEntity;
  /** 取得に失敗したら null(警告に記録)。 */
  Branches: ClientBranchEntity[] | null;
  Contacts: ContactEntity[] | null;
}
export interface VendorResult extends WithUrl {
  Vendor: VendorEntity;
  Branches: VendorBranchEntity[] | null;
  Contacts: VendorContactEntity[] | null;
}
export interface ProjectResult extends WithUrl {
  Project: ProjectEntity;
  Client: ClientEntity | null;
}
export interface UserResult {
  User: UserEntity;
}
interface DocumentResultBase extends WithUrl {
  /** 逆引きで解決した案件 ID。解決できなければ 0。 */
  ProjectID: number;
  ClientID: number;
  Project: ProjectEntity | null;
  Client: ClientEntity | null;
}
export type EstimateResult = DocumentResultBase & { Estimate: EstimateEntity };
export type OrderResult = DocumentResultBase & { Order: OrderEntity };
export type DeliveryResult = DocumentResultBase & { Delivery: DeliveryEntity };
export type ReceiptResult = DocumentResultBase & { Receipt: ReceiptEntity };
export interface InvoiceResult extends WithUrl {
  Invoice: InvoiceEntity;
  Project: ProjectEntity | null;
  Client: ClientEntity | null;
}
export interface PurchaseOrderResult extends WithUrl {
  PurchaseOrder: PurchaseOrderEntity;
  Vendor: VendorEntity | null;
  Project: ProjectEntity | null;
}
export interface PaymentResult extends WithUrl {
  Payment: PaymentEntity;
  Vendor: VendorEntity | null;
  /** 支払は案件 ID を持たないので常に null(Go と同じ)。 */
  Project: null;
}
