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

/**
 * ユーザー。GET /v1/users の応答 1 要素。
 * BOARD の OpenAPI(v1.9.0)の応答項目だけを持つ。姓名は `last_name` / `first_name` に分かれていて、`name` は無い(issue #27)。
 */
export interface UserEntity {
  id: number;
  email: string;
  last_name: string;
  first_name: string;
  role_id: number;
  role_name: string;
  last_sign_in_at: string | null;
  valid_flg: number;
  created_at: string;
  updated_at: string;
}

/**
 * GET /v1/users のクエリ。BOARD の API が受けるのは `email_eq` だけで、名前・メールの部分一致の絞り込みは無い。
 * `nameCont` / `emailCont` は**キャッシュ専用のキー**(API には送らない)。repository が全件を取ってから `match` で絞る。
 */
export interface UserListOptions extends CommonListOptions {
  /** 姓・名・姓名の部分一致(キャッシュ専用。API には送らない) */
  nameCont?: string;
  /** メールアドレスの部分一致(キャッシュ専用。API には送らない) */
  emailCont?: string;
}

export function buildUsersQuery(opts: UserListOptions, page: number, perPage: number): string {
  return baseQuery("users", opts, page, perPage).encode();
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
