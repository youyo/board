import type { BoardClient } from "../board/client";
import type { CallContext } from "../board/context";
import {
  type CommonListOptions,
  type ItemResult,
  type ListPaging,
  type ListResult,
  RESOURCES,
} from "../board/resources";

/**
 * リソースごとの振る舞い。Go 版は 22 ファイルに同じ形を書き写していたが、違いは次の 3 種類と
 * 『キャッシュ側で絞れる項目と判定』だけなので、表として持つ。
 *
 * - cache-first(7): 毎回 refresh(指示があれば)→ ゼロフィルタはキャッシュ全件 / キャッシュ側で絞れるフィルタは
 *   同期済みならキャッシュを fold で絞る(未同期なら API)/ サーバー専用の項目があれば API。
 * - nonzero-api(11): 非ゼロのフィルタは refresh もキャッシュも使わず API。ゼロフィルタだけ refresh → キャッシュ全件。
 * - document(4): 一覧がない。get はキャッシュ → なければ API(refresh=full はキャッシュを使わず取り直す)。
 */
interface Base<E> {
  resource: string;
  /** `/v1/clients` など。 */
  path: string;
  get: (client: BoardClient, id: number, ctx?: CallContext) => Promise<ItemResult<E>>;
}

interface ListBase<E, F extends CommonListOptions> extends Base<E> {
  buildQuery: (opts: F, page: number, perPage: number) => string;
  list: (
    client: BoardClient,
    opts: F,
    paging?: ListPaging,
    ctx?: CallContext,
  ) => Promise<ListResult<E>>;
}

export interface CacheFirstSpec<E, F extends CommonListOptions> extends ListBase<E, F> {
  kind: "cache-first";
  cacheFirst: {
    /** キャッシュ側で絞れる項目。これ以外(page / perPage / updatedAt* / includeArchiveFlg / tags / responseGroup など)があれば API。 */
    keys: ReadonlyArray<keyof F & string>;
    match: (entity: E, filter: F) => boolean;
  };
}

export interface NonzeroApiSpec<E, F extends CommonListOptions> extends ListBase<E, F> {
  kind: "nonzero-api";
}

export interface DocumentSpec<E> extends Base<E> {
  kind: "document";
}

export type ListSpec<E, F extends CommonListOptions> = CacheFirstSpec<E, F> | NonzeroApiSpec<E, F>;

export function pathOf(resource: string): string {
  const r = RESOURCES.find((x) => x.name === resource);
  if (!r) throw new Error(`unknown resource: ${resource}`);
  return r.path;
}
