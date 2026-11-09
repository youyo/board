/**
 * repository 層: キャッシュ → refresh → API の統一的な参照窓口(Go 版 `internal/repository/*` を 1 つの汎用実装にしたもの)。
 *
 * - refresh は明示指示(`refresh: delta | full`)のときだけ。daily auto-refresh と初回の自動構築は廃止した。
 * - 公開関数は最後の任意引数に `ctx?: CallContext` を取る。refresh も API 呼び出しも同じ締め切りを共有する。
 * - 結果には、読んだリソースの鮮度情報(`cache`)を同梱する。
 */

import { type CallContext, resolveContext } from "../board/context";
import type { CommonListOptions } from "../board/resources";
import { assertId } from "../board/resources";
import { type CacheInfo, cacheInfoFromState } from "../cache/info";
import { cacheFetchedEntity, countEntries, getEntry, listEntries } from "../cache/resource-cache";
import { ensureSchema } from "../cache/schema";
import { getSyncState, hasSynced } from "../cache/sync-state";
import { systemClock } from "../cache/time";
import {
  type RefreshDeps,
  type RefreshOutcome,
  type RefreshTarget,
  refreshResource,
} from "../refresh/refresher";
import { isCacheableFilter, isZeroFilter, limitResult } from "./filter";
import { SPECS } from "./resources";
import type { CacheFirstSpec, DocumentSpec, ListSpec } from "./spec";
import type { GetReadResult, ListReadResult, ReadOptions } from "./types";

export type RepositoryDeps = RefreshDeps;

export interface ListRepository<E, F extends CommonListOptions> {
  readonly resource: string;
  list(filter?: F, readOpts?: ReadOptions, ctx?: CallContext): Promise<ListReadResult<E>>;
  /** list と同じ(find 層の Search の呼び名)。 */
  search(filter?: F, readOpts?: ReadOptions, ctx?: CallContext): Promise<ListReadResult<E>>;
  get(id: number, readOpts?: ReadOptions, ctx?: CallContext): Promise<GetReadResult<E>>;
  /** 明示的な refresh(キャッシュ系ツールや find 層が使う)。ロックが取れなければ RefreshInProgressError。 */
  refresh(mode: "delta" | "full", ctx?: CallContext): Promise<RefreshOutcome>;
}

export interface DocumentRepository<E> {
  readonly resource: string;
  get(id: number, readOpts?: ReadOptions, ctx?: CallContext): Promise<GetReadResult<E>>;
}

const outcomeInfo = (o: RefreshOutcome | undefined) =>
  o ? { state: o.state, errorCode: o.errorCode } : undefined;

function createListRepository<E, F extends CommonListOptions>(
  spec: ListSpec<E, F>,
  deps: RepositoryDeps,
): ListRepository<E, F> {
  const { resource } = spec;
  const now = deps.now ?? systemClock;
  const target: RefreshTarget = {
    resource,
    path: spec.path,
    buildQuery: (o, page, perPage) => spec.buildQuery(o as F, page, perPage),
  };

  const refresh = (mode: "delta" | "full", ctx: CallContext) =>
    refreshResource(deps, target, mode, ctx);

  async function maybeRefresh(
    readOpts: ReadOptions,
    ctx: CallContext,
  ): Promise<RefreshOutcome | undefined> {
    const choice = readOpts.refresh ?? "none";
    return choice === "none" ? undefined : refresh(choice, ctx);
  }

  async function info(outcome?: RefreshOutcome): Promise<CacheInfo> {
    return cacheInfoFromState(
      resource,
      await getSyncState(deps.db, resource),
      outcomeInfo(outcome),
    );
  }

  async function viaApi(
    filter: F,
    readOpts: ReadOptions,
    ctx: CallContext,
    outcome?: RefreshOutcome,
  ): Promise<ListReadResult<E>> {
    // キャッシュ専用のキー(users の name_cont など)は API に送らない。絞り込みなしで取ってから match で絞る。
    const cf = spec.kind === "cache-first" ? spec.cacheFirst : undefined;
    const cacheOnly = cf?.cacheOnlyKeys?.filter((k) => !isZeroFilter({ [k]: filter[k] })) ?? [];
    const apiFilter = { ...filter } as F;
    for (const k of cacheOnly) delete apiFilter[k];
    const r = await spec.list(deps.client, apiFilter, undefined, ctx);
    const items = cf && cacheOnly.length > 0 ? r.items.filter((e) => cf.match(e, filter)) : r.items;
    return {
      ...limitResult(items, readOpts.limit),
      source: "api",
      // refresh を走らせたうえで API にフォールバックしたときは、その結果(partial / failed)を落とさない。
      cache: outcome ? [await info(outcome)] : [],
      truncated: r.truncated,
      _meta: r._meta,
    };
  }

  async function list(filter: F = {} as F, readOpts: ReadOptions = {}, callCtx?: CallContext) {
    const ctx = resolveContext(callCtx, now);
    await ensureSchema(deps.db);
    const zero = isZeroFilter(filter);

    // nonzero-api: 非ゼロのフィルタは refresh もキャッシュも経由しない(Go と同じ)。
    if (spec.kind === "nonzero-api" && !zero) return viaApi(filter, readOpts, ctx);

    // cache-first: キャッシュでは扱えない絞り込み(サーバー専用の項目)が 1 つでもあれば、結果は必ず API から取る。
    // その場合 refresh は結果に効かない(通信が無駄になり、refresh_in_progress にもなりうる)ので、走らせない(M04 の指摘)。
    const cf = spec.kind === "cache-first" ? spec.cacheFirst : undefined;
    if (!zero && cf && !isCacheableFilter(filter, cf.keys)) return viaApi(filter, readOpts, ctx);

    const outcome = await maybeRefresh(readOpts, ctx);

    if (!zero) {
      // キャッシュで絞れる項目だけ: 同期済みなら fold で絞る。未同期(refresh が失敗・partial など)なら API。
      if (hasSynced(await getSyncState(deps.db, resource))) {
        try {
          const entries = await listEntries<E>(deps.db, resource);
          const items = entries
            .map((e) => e.entity)
            .filter((e) => (cf as CacheFirstSpec<E, F>["cacheFirst"]).match(e, filter));
          return {
            ...limitResult(items, readOpts.limit),
            source: "cache",
            cache: [await info(outcome)],
          } satisfies ListReadResult<E>;
        } catch {
          // 壊れた行がある: 安全側に倒して API に任せる(Go の tryCacheFilter と同じ)。
        }
      }
      return viaApi(filter, readOpts, ctx, outcome);
    }

    // ゼロフィルタ: キャッシュ全件。空なら空(未取得は cache の cached_at が無いことで分かる。API は呼ばない)。
    // limit があるときは SQL の LIMIT で必要な件数だけ読む。切ったかどうかは件数の照会で分かる(totalBeforeLimit)。
    const limit = readOpts.limit !== undefined && readOpts.limit > 0 ? readOpts.limit : undefined;
    const entries = await listEntries<E>(deps.db, resource, { limit });
    const items = entries.map((e) => e.entity);
    const total =
      limit !== undefined && items.length >= limit ? await countEntries(deps.db, resource) : 0;
    return {
      ...(total > items.length ? { items, totalBeforeLimit: total } : { items }),
      source: "cache",
      cache: [await info(outcome)],
    } satisfies ListReadResult<E>;
  }

  async function get(id: number, readOpts: ReadOptions = {}, callCtx?: CallContext) {
    assertId(id);
    const ctx = resolveContext(callCtx, now);
    await ensureSchema(deps.db);
    const outcome = await maybeRefresh(readOpts, ctx);

    const hit = await getEntry<E>(deps.db, resource, id);
    if (hit) {
      return {
        item: hit.entity,
        source: "cache",
        cache: [await info(outcome)],
      } satisfies GetReadResult<E>;
    }

    // キャッシュにない: API から 1 件取って保存する。世代の読み取り・ロックの確認・ID の照合は 1 文の中で行う。
    const fetchStartedAt = now();
    const res = await spec.get(deps.client, id, ctx);
    await cacheFetchedEntity(deps.db, resource, res.item, {
      requestedId: id,
      fetchStartedAt,
      now: now(),
    });
    return {
      item: res.item,
      source: "api",
      // refresh を走らせたあとのキャッシュミスは、その結果を落とさない。
      cache: outcome ? [await info(outcome)] : [],
      _meta: res._meta,
    } satisfies GetReadResult<E>;
  }

  return { resource, list, search: list, get, refresh };
}

function createDocumentRepository<E>(
  spec: DocumentSpec<E>,
  deps: RepositoryDeps,
): DocumentRepository<E> {
  const { resource } = spec;
  const now = deps.now ?? systemClock;

  async function get(id: number, readOpts: ReadOptions = {}, callCtx?: CallContext) {
    assertId(id);
    const ctx = resolveContext(callCtx, now);
    await ensureSchema(deps.db);
    // refresh=full はキャッシュを使わず取り直す(Go の ForceRefresh)。delta は document には意味がないので何もしない。
    if (readOpts.refresh !== "full") {
      const hit = await getEntry<E>(deps.db, resource, id);
      if (hit) {
        return {
          item: hit.entity,
          source: "cache",
          // document には同期状態がない。取得した時刻がそのまま鮮度。
          cache: [{ resource, cached_at: hit.fetched_at }],
        } satisfies GetReadResult<E>;
      }
    }
    const fetchStartedAt = now();
    const res = await spec.get(deps.client, id, ctx);
    await cacheFetchedEntity(deps.db, resource, res.item, {
      requestedId: id,
      fetchStartedAt,
      now: now(),
    });
    return {
      item: res.item,
      source: "api",
      cache: [],
      _meta: res._meta,
    } satisfies GetReadResult<E>;
  }

  return { resource, get };
}

export { createDocumentRepository, createListRepository, SPECS };
