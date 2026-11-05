import type { CallContext } from "../board/context";
import { resolveContext } from "../board/context";
import type { ProjectEntity, ProjectListOptions } from "../board/resources";
import { assertId, getProject } from "../board/resources";
import { systemClock } from "../cache/time";
import {
  createDocumentRepository,
  createListRepository,
  type DocumentRepository,
  type ListRepository,
  type RepositoryDeps,
} from "./repository";
import { SPECS, type Specs } from "./resources";
import type { CacheFirstSpec, DocumentSpec, ListSpec } from "./spec";
import type { GetReadResult } from "./types";

export type { CacheInfo } from "../cache/info";
export type { RefreshOutcome } from "../refresh/refresher";
export { applyLimit, isCacheableFilter, isZeroFilter, limitResult } from "./filter";
export type { DocumentRepository, ListRepository, RepositoryDeps } from "./repository";
export { SPECS } from "./resources";
export type { GetReadResult, ListReadResult, ReadOptions, RefreshChoice } from "./types";

type RepoOf<S> =
  S extends ListSpec<infer E, infer F>
    ? ListRepository<E, F>
    : S extends DocumentSpec<infer E>
      ? DocumentRepository<E>
      : never;

type BaseRepositories = { [K in keyof Specs]: RepoOf<Specs[K]> };

export type Repositories = Omit<BaseRepositories, "projects"> & {
  projects: ListRepository<ProjectEntity, ProjectListOptions> & {
    /** `response_group` 付きの 1 件取得。埋め込みをキャッシュから再構築できないので、常に API を直接呼ぶ。 */
    getWithGroup(
      id: number,
      responseGroup: string,
      ctx?: CallContext,
    ): Promise<GetReadResult<ProjectEntity>>;
  };
};

/** 22 リソースの repository。M04(api 層)・M05(find 層)がこれを使う。 */
export function createRepositories(deps: RepositoryDeps): Repositories {
  const out: Record<string, unknown> = {};
  for (const [name, spec] of Object.entries(SPECS)) {
    out[name] =
      spec.kind === "document"
        ? createDocumentRepository(spec as DocumentSpec<unknown>, deps)
        : createListRepository(spec as CacheFirstSpec<unknown, never>, deps);
  }
  const projects = out.projects as ListRepository<ProjectEntity, ProjectListOptions>;
  const now = deps.now ?? systemClock;
  out.projects = {
    ...projects,
    async getWithGroup(id: number, responseGroup: string, callCtx?: CallContext) {
      assertId(id);
      const res = await getProject(
        deps.client,
        id,
        { responseGroup },
        resolveContext(callCtx, now),
      );
      return { item: res.item, source: "api", cache: [], _meta: res._meta };
    },
  };
  return out as Repositories;
}
