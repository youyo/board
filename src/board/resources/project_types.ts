import type { BoardClient } from "../client";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";

/** 案件種別。GET /v1/project_types の応答 1 要素。 */
export interface ProjectTypeEntity {
  id: number;
  name: string;
  memo: string;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
  [key: string]: unknown;
}

/** GET /v1/project_types のクエリ(Ransack)。未指定の項目は送らない。 */
export interface ProjectTypeListOptions extends CommonListOptions {
  /** 案件種別名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildProjectTypesQuery(
  opts: ProjectTypeListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listProjectTypes(
  client: BoardClient,
  opts: ProjectTypeListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<ProjectTypeEntity>> {
  return listResource<ProjectTypeEntity, ProjectTypeListOptions>(
    client,
    "/v1/project_types",
    buildProjectTypesQuery,
    opts,
    paging,
  );
}

export function getProjectType(
  client: BoardClient,
  id: number,
): Promise<ItemResult<ProjectTypeEntity>> {
  return getResource<ProjectTypeEntity>(client, "/v1/project_types", id);
}
