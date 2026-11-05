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

/** ユーザー。GET /v1/users の応答 1 要素(Go の UserEntity)。 */
export interface UserEntity {
  id: number;
  name: string;
  last_name: string;
  first_name: string;
  email: string;
  role_id: number;
  role_name: string;
  last_sign_in_at: string;
  valid_flg: number;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface UserListOptions extends CommonListOptions {
  /** ユーザー名の部分一致(name_cont) */
  nameCont?: string;
  /** メールアドレスの部分一致(email_cont) */
  emailCont?: string;
}

export function buildUsersQuery(opts: UserListOptions, page: number, perPage: number): string {
  return baseQuery(opts, page, perPage)
    .strCont("name", opts.nameCont)
    .strCont("email", opts.emailCont)
    .encode();
}

export function listUsers(
  client: BoardClient,
  opts: UserListOptions = {},
  paging?: ListPaging,
  ctx?: CallContext,
): Promise<ListResult<UserEntity>> {
  return listResource<UserEntity, UserListOptions>(
    client,
    "/v1/users",
    buildUsersQuery,
    opts,
    paging,
    ctx,
  );
}

export function getUser(
  client: BoardClient,
  id: number,
  ctx?: CallContext,
): Promise<ItemResult<UserEntity>> {
  return getResource<UserEntity>(client, "/v1/users", id, undefined, "id", ctx);
}
