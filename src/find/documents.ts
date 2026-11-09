/**
 * 書類 4 種(見積・受注・納品・領収)の検索(Go 版 `find_estimate.go` `find_order.go` `find_delivery.go` `find_receipt.go`)。
 * 4 種は構造が同じなので、種別の表(`DOCUMENT_KIND_INFO`)で 1 つの実装にした。
 *
 * 書類のエンティティは案件 ID・顧客 ID を持たない。経路ごとに案件を取り、案件に埋め込まれた書類 ID から書類を 1 件ずつ取る:
 *  - id: 書類を取り、逆引き(`DocumentIndex`)で案件 ID を解決する。解決できなければ ProjectID=0 + 警告(縮退)。
 *  - project_id: `response_group=<種別>` 付きで案件を取る。
 *  - client_name / project_name: 名前にマッチした全件に fanout する(曖昧エラーにはしない)。5 件を超えたら拒否。
 * 優先順位は id > project_id > client_name > project_name。client_name と project_name は AND(顧客の案件のうち名前が合うもの)。
 */
import type { ClientEntity, ProjectEntity } from "../board/resources";
import { BoardError } from "../errors";
import { DOCUMENT_KIND_INFO, type DocumentKind } from "../repository/document-kinds";
import {
  errFanoutTooMany,
  errTooManyDocuments,
  FANOUT_RESOLVE_CAP,
  FIND_DOCUMENT_CANDIDATE_CAP,
} from "./fanout";
import type { FindSession } from "./session";
import {
  type DeliveryResult,
  type DocumentQuery,
  type EstimateResult,
  type OrderResult,
  type ReceiptResult,
  validateDocumentQuery,
} from "./types";
import { documentUrl } from "./url";

/** 結果のキー(Go の Result 構造体のフィールド名)。 */
const RESULT_KEY: Record<DocumentKind, "Estimate" | "Order" | "Delivery" | "Receipt"> = {
  estimate: "Estimate",
  order: "Order",
  delivery: "Delivery",
  receipt: "Receipt",
};

interface Resolved {
  ProjectID: number;
  ClientID: number;
  Project: ProjectEntity | null;
  Client: ClientEntity | null;
}
type AnyDocumentResult = Resolved & { url?: string } & Record<string, unknown>;

interface Candidate {
  documentId: number;
  project: ProjectEntity;
  clientId: number;
  /** すでに分かっているときだけ(client_name の経路)。なければ補助情報として取る。 */
  client?: ClientEntity;
}

/** response_group=<種別> 付きの案件で、その種別の書類が埋め込まれるキー(案件のキーは deliveries / receipts と複数形)。 */
const EMBED_KEY: Record<DocumentKind, "estimate" | "order" | "deliveries" | "receipts"> = {
  estimate: "estimate",
  order: "order",
  delivery: "deliveries",
  receipt: "receipts",
};

/**
 * 結果の Project に入れる案件。response_group で埋め込まれた書類本体は、結果の書類側(Estimate など)と
 * 二重になって応答が倍になるので落とす。元の案件(逆引きの記録などが読む)は書き換えない。
 */
function projectForResult(kind: DocumentKind, p: ProjectEntity): ProjectEntity {
  const key = EMBED_KEY[kind];
  if (!(key in p)) return p;
  const { [key]: _embedded, ...rest } = p;
  return rest as ProjectEntity;
}

const isNotFound = (e: unknown) => e instanceof BoardError && e.code === "not_found";

function build(
  kind: DocumentKind,
  s: FindSession,
  document: unknown,
  rest: Resolved,
): AnyDocumentResult {
  const url = documentUrl(s.uiBaseUrl, rest.ProjectID);
  return {
    [RESULT_KEY[kind]]: document,
    ProjectID: rest.ProjectID,
    ClientID: rest.ClientID,
    Project: rest.Project,
    Client: rest.Client,
    ...(url === "" ? {} : { url }),
  };
}

async function findById(
  kind: DocumentKind,
  s: FindSession,
  id: number,
): Promise<AnyDocumentResult[]> {
  const info = DOCUMENT_KIND_INFO[kind];
  const doc = await s.get(info.resource, id);

  let lookup: Awaited<ReturnType<typeof s.repos.documentIndex.lookup>>;
  try {
    lookup = await s.repos.documentIndex.lookup(kind, id, s.ctx);
  } catch (e) {
    lookup = {
      status: "unresolved",
      reason: "api_error",
      errorCode: e instanceof BoardError ? e.code : "generic_error",
    };
  }
  if (lookup.status !== "hit") {
    // 縮退: 案件を解決できなかった。書類は返し、ProjectID=0 + 警告で伝える。
    s.warn({
      code: "project_unresolved",
      resource: info.resource,
      document_id: id,
      reason: lookup.reason,
      ...(lookup.status === "unresolved" && lookup.errorCode
        ? { error_code: lookup.errorCode }
        : {}),
      message:
        lookup.status === "miss"
          ? "No project containing this document was found; ProjectID is 0."
          : "The project of this document could not be resolved in time; ProjectID is 0. Retry: the lookup resumes from the saved progress.",
    });
    return [build(kind, s, doc, { ProjectID: 0, ClientID: 0, Project: null, Client: null })];
  }

  const pid = lookup.projectId;
  const Project = await s.getOptional("projects", pid);
  // 案件を取れなければ ProjectID だけ返す(Go と同じ)。
  if (!Project)
    return [build(kind, s, doc, { ProjectID: pid, ClientID: 0, Project: null, Client: null })];
  const cid = Project.client?.id ?? 0;
  const Client = cid ? await s.getOptional("clients", cid) : null;
  return [build(kind, s, doc, { ProjectID: pid, ClientID: cid, Project, Client })];
}

/** 候補の書類を 1 件ずつ取って結果にする。404 は飛ばす。limit に達したら止める。 */
async function fetchAll(
  kind: DocumentKind,
  s: FindSession,
  cands: Candidate[],
  limit: number | undefined,
  /** 候補の収集を limit で打ち切った(まだ見ていない顧客がある)。このとき候補数は下限でしかない。 */
  truncated = false,
): Promise<AnyDocumentResult[]> {
  const info = DOCUMENT_KIND_INFO[kind];
  const out: AnyDocumentResult[] = [];
  for (const [i, c] of cands.entries()) {
    let doc: unknown;
    try {
      doc = await s.get(info.resource, c.documentId);
    } catch (e) {
      if (isNotFound(e)) continue;
      throw e;
    }
    const Client = c.client ?? (c.clientId ? await s.getOptional("clients", c.clientId) : null);
    out.push(
      build(kind, s, doc, {
        ProjectID: c.project.id,
        ClientID: c.clientId,
        Project: projectForResult(kind, c.project),
        Client,
      }),
    );
    if (limit && limit > 0 && out.length >= limit) {
      if (i < cands.length - 1 || truncated) s.totalBeforeLimit = cands.length;
      break;
    }
  }
  return out;
}

function candidatesOf(
  kind: DocumentKind,
  projects: ProjectEntity[],
  client?: ClientEntity,
): Candidate[] {
  const info = DOCUMENT_KIND_INFO[kind];
  return projects.flatMap((project) =>
    info.ids(project).map((documentId) => ({
      documentId,
      project,
      clientId: client?.id ?? project.client?.id ?? 0,
      ...(client ? { client } : {}),
    })),
  );
}

async function findDocuments(
  kind: DocumentKind,
  s: FindSession,
  q: DocumentQuery,
): Promise<AnyDocumentResult[]> {
  validateDocumentQuery(q);
  if (q.id) return findById(kind, s, q.id);

  if (q.projectId) {
    const p = await s.projectWithGroup(q.projectId, kind);
    return fetchAll(kind, s, candidatesOf(kind, [p]), q.limit);
  }

  if (q.clientName) {
    const clients = await s.search("clients", { nameCont: q.clientName });
    if (clients.length > FANOUT_RESOLVE_CAP) {
      throw errFanoutTooMany("client_name", q.clientName, clients.length);
    }
    const cands: Candidate[] = [];
    let truncated = false;
    for (const c of clients) {
      // limit に達したら、残りの顧客の案件は取りに行かない(候補数はここまでに見つけた分)。
      if (q.limit && q.limit > 0 && cands.length >= q.limit) {
        truncated = true;
        break;
      }
      const projects = await s.search("projects", {
        clientIdEq: c.id,
        ...(q.projectName ? { nameCont: q.projectName } : {}),
        responseGroup: kind,
      });
      await s.recordProjects(kind, projects);
      cands.push(...candidatesOf(kind, projects, c));
      // 書類を取りに行く前に拒否する(limit で足りる場合は上の打ち切りが先に効く)。
      if (cands.length > FIND_DOCUMENT_CANDIDATE_CAP) {
        throw errTooManyDocuments("client_name", q.clientName);
      }
    }
    return fetchAll(kind, s, cands, q.limit, truncated);
  }

  // project_name だけ
  const projects = await s.search("projects", { nameCont: q.projectName, responseGroup: kind });
  if (projects.length > FANOUT_RESOLVE_CAP) {
    throw errFanoutTooMany("project_name", q.projectName ?? "", projects.length);
  }
  await s.recordProjects(kind, projects);
  const cands = candidatesOf(kind, projects);
  if (cands.length > FIND_DOCUMENT_CANDIDATE_CAP) {
    throw errTooManyDocuments("project_name", q.projectName ?? "");
  }
  return fetchAll(kind, s, cands, q.limit);
}

export const findEstimates = (s: FindSession, q: DocumentQuery) =>
  findDocuments("estimate", s, q) as unknown as Promise<EstimateResult[]>;
export const findOrders = (s: FindSession, q: DocumentQuery) =>
  findDocuments("order", s, q) as unknown as Promise<OrderResult[]>;
export const findDeliveries = (s: FindSession, q: DocumentQuery) =>
  findDocuments("delivery", s, q) as unknown as Promise<DeliveryResult[]>;
export const findReceipts = (s: FindSession, q: DocumentQuery) =>
  findDocuments("receipt", s, q) as unknown as Promise<ReceiptResult[]>;
