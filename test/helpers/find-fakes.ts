import { vi } from "vitest";
import { BoardApiError } from "../../src/board/errors";
import type { CacheInfo } from "../../src/cache/info";
import { type FindRepos, FindSession } from "../../src/find/session";
import type { RefreshChoice } from "../../src/repository";
import type { IndexLookup } from "../../src/repository/document-index";

// biome-ignore lint/suspicious/noExplicitAny: 偽の repository はテストごとに任意の形のエンティティを返す
type Any = any;

export interface ResourceFake {
  list?: (filter: Any, readOpts: Any) => Any[] | Promise<Any[]>;
  get?: (id: number, readOpts: Any) => Any | Promise<Any>;
}

const RESOURCE_NAMES = [
  "clients",
  "client_branches",
  "contacts",
  "projects",
  "estimates",
  "orders",
  "deliveries",
  "receipts",
  "invoices",
  "vendors",
  "vendor_branches",
  "vendor_contacts",
  "purchase_orders",
  "payments",
  "users",
] as const;

export type Fakes = { [K in (typeof RESOURCE_NAMES)[number]]?: ResourceFake } & {
  projectsWithGroup?: (id: number, group: string) => Any | Promise<Any>;
  lookup?: (kind: string, id: number) => IndexLookup | Promise<IndexLookup>;
};

export const apiError = (status = 500) => new BoardApiError(status, {});
export const notFound = () => new BoardApiError(404, {});

/**
 * find 層のための偽の repository。ハンドラの定義がない呼び出しは例外にする(想定外の通信を検出する)。
 * `log` に `<resource>.list|get <引数>` を残す。結果の cache は {resource} だけ(キャッシュから返したことにする)。
 */
export function fakeFind(fakes: Fakes = {}, opts: { refresh?: RefreshChoice } = {}) {
  const log: string[] = [];
  const unexpected = (what: string) => () => {
    throw new Error(`fake: 想定外の呼び出し ${what}`);
  };
  const resourceRepo = (resource: string) => {
    const f = (fakes as Record<string, ResourceFake | undefined>)[resource] ?? {};
    const cache: CacheInfo[] = [{ resource }];
    const list = async (filter: Any = {}, readOpts: Any = {}) => {
      log.push(`${resource}.list ${JSON.stringify(filter)} ${JSON.stringify(readOpts)}`);
      const items = await (f.list ?? unexpected(`${resource}.list`))(filter, readOpts);
      return { items, source: "cache", cache };
    };
    const get = async (id: number, readOpts: Any = {}) => {
      log.push(`${resource}.get ${id} ${JSON.stringify(readOpts)}`);
      const item = await (f.get ?? unexpected(`${resource}.get`))(id, readOpts);
      return { item, source: "cache", cache };
    };
    return { resource, list, search: list, get, refresh: unexpected(`${resource}.refresh`) };
  };
  const repos: Record<string, Any> = {};
  for (const n of RESOURCE_NAMES) repos[n] = resourceRepo(n);
  repos.projects.getWithGroup = async (id: number, group: string) => {
    log.push(`projects.getWithGroup ${id} ${group}`);
    const item = await (fakes.projectsWithGroup ?? unexpected("projects.getWithGroup"))(id, group);
    return { item, source: "api", cache: [] };
  };
  const lookup = vi.fn(async (kind: string, id: number): Promise<IndexLookup> => {
    log.push(`index.lookup ${kind} ${id}`);
    return (await (fakes.lookup ?? unexpected("index.lookup"))(kind, id)) as IndexLookup;
  });
  const record = vi.fn(async (kind: string, projects: Any[]) => {
    log.push(`index.record ${kind} ${projects.map((p) => p.id).join(",")}`);
  });
  repos.documentIndex = { lookup, record };

  const session = (o: { limit?: number; refresh?: RefreshChoice } = {}) =>
    new FindSession({
      repos: repos as FindRepos,
      ctx: { deadline: Date.now() + 30_000 },
      refresh: o.refresh ?? opts.refresh ?? "none",
      uiBaseUrl: "https://the-board.jp",
    });
  const calls = (prefix: string) => log.filter((l) => l.startsWith(prefix));
  return { log, calls, session, lookup, record };
}

/** id を持つエンティティ n 件(name: `${prefix}${i}`)。 */
export const entities = (n: number, prefix: string, extra: (i: number) => object = () => ({})) =>
  Array.from({ length: n }, (_, i) => ({ id: i + 1, name: `${prefix}${i + 1}`, ...extra(i + 1) }));
