import type { BoardClient } from "../client";
import type { CallContext } from "../context";
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
  archive_flg?: number;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
}

/** GET /v1/project_types のクエリ(Ransack)。未指定の項目は送らない。 */
export interface ProjectTypeListOptions extends CommonListOptions {
  /** 案件種別名の部分一致(name_cont) */
  nameCont?: string;
  /** 案件区分1〜3の種類(1〜3。project_type_kbn_eq)。未指定は 1 と同じ結果。サーバー専用(キャッシュでは絞らない) */
  projectTypeKbnEq?: number;
}

export function buildProjectTypesQuery(
  opts: ProjectTypeListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery("project_types", opts, page, perPage)
    .strCont("name", opts.nameCont)
    .intEq("project_type_kbn", opts.projectTypeKbnEq)
    .encode();
}

export function listProjectTypes(
  client: BoardClient,
  opts: ProjectTypeListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<ProjectTypeEntity>> {
  return listResource<ProjectTypeEntity, ProjectTypeListOptions>(
    client,
    "/v1/project_types",
    buildProjectTypesQuery,
    opts,
    paging,
    ctx,
  );
}

export function getProjectType(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<ProjectTypeEntity>> {
  return getResource<ProjectTypeEntity>(client, "/v1/project_types", id, undefined, "id", ctx);
}
