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

/** グループ。GET /v1/groups の応答 1 要素。 */
export interface GroupEntity {
  id: number;
  name: string;
  archive_flg?: number;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
}

/** GET /v1/groups のクエリ(Ransack)。未指定の項目は送らない。 */
export interface GroupListOptions extends CommonListOptions {
  /** グループ名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildGroupsQuery(opts: GroupListOptions, page: number, perPage: number): string {
  return baseQuery("groups", opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listGroups(
  client: BoardClient,
  opts: GroupListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<GroupEntity>> {
  return listResource<GroupEntity, GroupListOptions>(
    client,
    "/v1/groups",
    buildGroupsQuery,
    opts,
    paging,
    ctx,
  );
}

export function getGroup(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<GroupEntity>> {
  return getResource<GroupEntity>(client, "/v1/groups", id, undefined, "id", ctx);
}
