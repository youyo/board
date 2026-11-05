/**
 * find 層の 1 回のツール呼び出しの状態(`CallContext` は 1 つ。締め切りは全体で共有する)。
 *
 * - repository(M03)からだけ読む。find 層で独自のキャッシュは持たない(キャッシュ責務は repository 側)。
 *   ただし 1 回の呼び出しの中で同じ ID を何度も引かないよう、取得結果をこの呼び出しの間だけ覚える(メモ)。
 * - `refresh` はリソースごとに最初の 1 回の読み取りにだけ付ける(列挙のたびに refresh を走らせない)。
 *   名前の解決と書類の取得は例外: 解決は refresh なし(Go と同じ)、書類は 1 件ごとに効く(full = 取り直し)。
 * - 補助情報(enrichment)の失敗は致命的にしない: その項目は null にして警告を記録する。
 *   警告にはエラーコードと対象 ID だけを載せ、例外の文面は載せない。
 */
import type { CallContext } from "../board/context";
import type { ProjectEntity } from "../board/resources";
import type { CacheInfo } from "../cache/info";
import { BoardError } from "../errors";
import type {
  GetReadResult,
  ListReadResult,
  ReadOptions,
  RefreshChoice,
  Repositories,
} from "../repository";
import type { DocumentIndex } from "../repository/document-index";
import type { DocumentKind } from "../repository/document-kinds";

/** find 層が使う repository。実物の `Repositories` の部分集合 + 書類 ID → 案件 ID の逆引き。 */
export type FindRepos = Pick<
  Repositories,
  | "clients"
  | "client_branches"
  | "contacts"
  | "projects"
  | "estimates"
  | "orders"
  | "deliveries"
  | "receipts"
  | "invoices"
  | "vendors"
  | "vendor_branches"
  | "vendor_contacts"
  | "purchase_orders"
  | "payments"
  | "users"
> & { documentIndex: DocumentIndex };

type ListKey = {
  [K in keyof FindRepos]: FindRepos[K] extends { list: unknown } ? K : never;
}[keyof FindRepos];
type GetKey = Exclude<keyof FindRepos, "documentIndex">;
const DOCUMENT_RESOURCES: ReadonlySet<string> = new Set([
  "estimates",
  "orders",
  "deliveries",
  "receipts",
]);

type Repo<K extends GetKey> = FindRepos[K];
export type EntityOf<K extends GetKey> = Awaited<ReturnType<Repo<K>["get"]>>["item"];
type FilterOf<K extends ListKey> = NonNullable<Parameters<Repo<K>["list"]>[0]>;

export type Warning =
  | { code: "enrichment_failed"; resource: string; id: number; error_code: string }
  | {
      code: "project_unresolved";
      resource: string;
      document_id: number;
      reason: string;
      error_code?: string;
      message: string;
    }
  | { code: "list_truncated"; resource: string; message: string }
  | { code: "warnings_truncated"; omitted: number };

/** 警告の最大件数(件数の多い一覧で応答が警告だらけにならないように)。 */
export const MAX_WARNINGS = 20;

export interface FindSessionOptions {
  repos: FindRepos;
  ctx: CallContext;
  refresh?: RefreshChoice;
  uiBaseUrl?: string;
}

const errorCodeOf = (e: unknown): string => (e instanceof BoardError ? e.code : "generic_error");

export class FindSession {
  readonly repos: FindRepos;
  readonly ctx: CallContext;
  readonly uiBaseUrl: string;
  private readonly refreshChoice: RefreshChoice;
  private readonly refreshed = new Set<string>();
  private readonly memo = new Map<string, Promise<unknown>>();
  private readonly warningList: Warning[] = [];
  private omittedWarnings = 0;
  private readonly cacheMap = new Map<string, CacheInfo>();
  /** limit で件数を切ったときの切る前の件数(切っていなければ undefined)。 */
  totalBeforeLimit: number | undefined;

  constructor(opts: FindSessionOptions) {
    this.repos = opts.repos;
    this.ctx = opts.ctx;
    this.refreshChoice = opts.refresh ?? "none";
    this.uiBaseUrl = opts.uiBaseUrl ?? "";
  }

  // --- 結果に載せるもの ---------------------------------------------------------------------

  get warnings(): Warning[] {
    return this.omittedWarnings > 0
      ? [...this.warningList, { code: "warnings_truncated", omitted: this.omittedWarnings }]
      : [...this.warningList];
  }

  /** 読んだリソースの鮮度情報(repository の結果の `cache` を、初めて出た順にまとめたもの)。 */
  get cache(): CacheInfo[] {
    return [...this.cacheMap.values()];
  }

  warn(w: Warning): void {
    if (this.warningList.length >= MAX_WARNINGS) this.omittedWarnings++;
    else this.warningList.push(w);
  }

  private note(infos: CacheInfo[]): void {
    for (const i of infos)
      this.cacheMap.set(i.resource, { ...this.cacheMap.get(i.resource), ...i });
  }

  /** limit で切る。切ったときは `totalBeforeLimit` を記録する。 */
  applyLimit<T>(items: T[], limit: number | undefined): T[] {
    if (limit === undefined || limit <= 0 || items.length <= limit) return items;
    this.totalBeforeLimit = items.length;
    return items.slice(0, limit);
  }

  // --- repository ---------------------------------------------------------------------------

  /** refresh は、そのリソースの最初の読み取りにだけ付ける。書類は 1 件ごと(refresh=full は取り直し)。 */
  private readOptions(resource: string, noRefresh?: boolean): ReadOptions {
    if (noRefresh || this.refreshChoice === "none") return {};
    if (DOCUMENT_RESOURCES.has(resource)) return { refresh: this.refreshChoice };
    if (this.refreshed.has(resource)) return {};
    this.refreshed.add(resource);
    return { refresh: this.refreshChoice };
  }

  async search<K extends ListKey>(
    key: K,
    filter: FilterOf<K>,
    o: { noRefresh?: boolean } = {},
  ): Promise<EntityOf<K>[]> {
    const repo = this.repos[key] as unknown as {
      list(f: unknown, r: ReadOptions, c: CallContext): Promise<ListReadResult<unknown>>;
    };
    const r = await repo.list(filter, this.readOptions(key, o.noRefresh), this.ctx);
    this.note(r.cache);
    if (r.truncated) {
      this.warn({
        code: "list_truncated",
        resource: key,
        message: "The list from BOARD was cut at the page limit; narrow the query to see the rest.",
      });
    }
    return r.items as EntityOf<K>[];
  }

  /** 主体の 1 件取得。失敗は呼び出し元に伝える(fail-fast)。 */
  async get<K extends GetKey>(
    key: K,
    id: number,
    o: { noRefresh?: boolean } = {},
  ): Promise<EntityOf<K>> {
    const repo = this.repos[key] as unknown as {
      get(i: number, r: ReadOptions, c: CallContext): Promise<GetReadResult<unknown>>;
    };
    const r = await repo.get(id, this.readOptions(key, o.noRefresh), this.ctx);
    this.note(r.cache);
    return r.item as EntityOf<K>;
  }

  /** 案件の 1 件取得(response_group 付き)。常に API を直接呼ぶ(M03)。含まれる書類 ID は逆引きに記録する。失敗は伝える。 */
  async projectWithGroup(id: number, kind: DocumentKind): Promise<ProjectEntity> {
    const r = await this.repos.projects.getWithGroup(id, kind, this.ctx);
    await this.recordProjects(kind, [r.item]);
    return r.item;
  }

  /** response_group 付きで取った案件の書類 ID を逆引きに覚える。記録の失敗で検索を止めない。 */
  async recordProjects(kind: DocumentKind, projects: readonly ProjectEntity[]): Promise<void> {
    try {
      await this.repos.documentIndex.record(kind, projects);
    } catch {
      // 逆引きの蓄積は最適化。失敗しても結果には影響しない。
    }
  }

  /** 同じ呼び出しの中で同じ (リソース, ID) を 1 度だけ取る補助情報の取得。失敗は null + 警告。 */
  getOptional<K extends GetKey>(key: K, id: number): Promise<EntityOf<K> | null> {
    const memoKey = `${key}:${id}`;
    let p = this.memo.get(memoKey) as Promise<EntityOf<K> | null> | undefined;
    if (!p) {
      p = this.soft({ resource: key, id }, () => this.get(key, id));
      this.memo.set(memoKey, p);
    }
    return p;
  }

  /** 補助情報の取得。失敗は致命的にせず null を返し、警告を記録する。 */
  async soft<T>(target: { resource: string; id: number }, fn: () => Promise<T>): Promise<T | null> {
    try {
      return await fn();
    } catch (e) {
      this.warn({ code: "enrichment_failed", ...target, error_code: errorCodeOf(e) });
      return null;
    }
  }

  /** 補助情報の一覧(支店・担当者)。失敗は null + 警告。 */
  async softList<K extends ListKey>(
    key: K,
    filter: FilterOf<K>,
    id: number,
  ): Promise<EntityOf<K>[] | null> {
    return this.soft({ resource: key, id }, () => this.search(key, filter, { noRefresh: true }));
  }
}
