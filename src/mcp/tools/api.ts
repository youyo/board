/**
 * api 層のツール(M04): 22 リソースの list(18)と get(22)。表(`api-resources.ts`)から作る。
 *
 * - 出力: list は `{ items, cache, truncated?, _meta? }`、get は `{ item, cache, _meta? }`。
 *   `_meta` は `show_meta=true` のときだけ付ける。BOARD の生データは BOARD のキーのまま返す。
 * - ツール呼び出しごとに 1 つの `CallContext`(MCP の abort signal + 既定 30 秒の締め切り)を作り、
 *   refresh・BOARD への通信・ページ巡回のすべてで共有する。
 * - BOARD への通信回数の上限(logbutler の `maxApiCalls`)は持たない: 呼び出し単位の締め切りと、
 *   1 クライアントでの直列実行が上限の役割を果たす。
 */
import { z } from "zod";
import type { CallContext } from "../../board/context";
import type { GetReadResult, ListReadResult, ReadOptions, RefreshChoice } from "../../repository";
import {
  idArg,
  optBool,
  optEnum,
  optInt,
  optIntArray,
  optNonNegativeInt,
  optStr,
  optStrArray,
  REFRESH_CHOICES,
} from "../args";
import { callContextOf } from "../call-context";
import { defineTool, type ToolDefinition } from "../registry";
import {
  API_RESOURCES,
  COMMON_FILTERS,
  type FieldDef,
  RESPONSE_GROUPS_PROJECTS_GET,
  type ResourceDef,
} from "./api-resources";

// --- 入力スキーマ --------------------------------------------------------------------------

function fieldSchema(f: FieldDef): z.ZodType {
  switch (f.kind.type) {
    case "str":
      return optStr(f.description);
    case "int":
      return optInt(f.description);
    case "bool":
      return optBool(f.description);
    case "strs":
      return optStrArray(f.description);
    case "ints":
      return optIntArray(f.description);
    case "enum":
      return optEnum(f.kind.values, f.description);
  }
}

const REFRESH_LIST_DESC =
  "キャッシュの更新指示: none=更新しない(既定。キャッシュの cached_at を見て判断する)/ delta=差分だけ取る(前回の同期以降の更新分)/ full=全件を取り直す(件数が多く 1 回で終わらないときは refresh_state=partial で返るので、もう一度 refresh=full を呼ぶと続きから進む)";
const LIMIT_DESC =
  "返す件数の上限(0 は無制限。絞り込みのあとに先頭から切る)。件数を切ったときだけ、応答に limited: true と total_before_limit(切る前の件数)が付く。truncated(BOARD のページ巡回の打ち切り)とは別の意味";
const SHOW_META_DESC =
  "true で応答に _meta(BOARD の応答ヘッダー由来: total_count / rate_limit_remaining / etag など)を付ける。BOARD から直接取ったときだけ値が入る";

/** list の入力: Go 版 CLI のフィルタフラグ + refresh / limit / show_meta(max_response_tokens は defineTool が足す)。 */
function listShape(def: ResourceDef): Record<string, z.ZodType> {
  const shape: Record<string, z.ZodType> = {};
  for (const f of [...def.fields, ...COMMON_FILTERS]) shape[f.arg] = fieldSchema(f);
  shape.refresh = optEnum(REFRESH_CHOICES, REFRESH_LIST_DESC);
  shape.limit = optNonNegativeInt(LIMIT_DESC);
  shape.show_meta = optBool(SHOW_META_DESC);
  return shape;
}

// --- 引数 → repository のフィルタ ------------------------------------------------------------

/** ツールの引数から `*ListOptions` を作る(指定された項目だけ。undefined は入れない)。 */
export function toListFilter(
  def: ResourceDef,
  args: Record<string, unknown>,
): Record<string, unknown> {
  const filter: Record<string, unknown> = {};
  for (const f of [...def.fields, ...COMMON_FILTERS]) {
    const v = args[f.arg];
    if (v !== undefined) filter[f.opt] = v;
  }
  return filter;
}

// --- 呼び出し ------------------------------------------------------------------------------

interface CommonArgs {
  refresh?: RefreshChoice;
  limit?: number;
  show_meta?: boolean;
}

const readOptions = (a: CommonArgs): ReadOptions => ({
  ...(a.refresh ? { refresh: a.refresh } : {}),
  ...(a.limit !== undefined ? { limit: a.limit } : {}),
});

function listOutput(r: ListReadResult<unknown>, showMeta: boolean | undefined) {
  return {
    items: r.items,
    cache: r.cache,
    ...(r.truncated ? { truncated: true } : {}),
    ...(r.totalBeforeLimit !== undefined
      ? { limited: true, total_before_limit: r.totalBeforeLimit }
      : {}),
    ...(showMeta && r._meta ? { _meta: r._meta } : {}),
  };
}

function getOutput(r: GetReadResult<unknown>, showMeta: boolean | undefined) {
  return {
    item: r.item,
    cache: r.cache,
    ...(showMeta && r._meta ? { _meta: r._meta } : {}),
  };
}

// --- 説明文 --------------------------------------------------------------------------------

function listDescription(def: ResourceDef): string {
  const head = `${def.label}の一覧を返す。${def.summary}。`;
  const common =
    "結果は {items, cache, truncated?, limited?, total_before_limit?, _meta?} で、items は BOARD の生データ。cache は読んだキャッシュの鮮度(cached_at=最終同期、full_refreshed_at=最終の全件取得、refresh_state)で、cached_at がなければ未取得を表す。";
  if (def.mode === "cache-first") {
    const keys = def.fields.filter((f) => f.cacheable).map((f) => f.arg);
    return (
      `${head}絞り込みなしなら、キャッシュ(D1)の全件を返し、BOARD は呼ばない(未取得なら items は空)。` +
      `最新が要るときは refresh=delta(差分)か refresh=full(全件)を指定する。` +
      `件数が多いリソースでは全件を返すと応答が大きくなるので、まず limit か絞り込みを指定する。` +
      `絞り込み: ${keys.join(" / ")} だけを指定した場合は、同期済みならキャッシュを絞って返す。それ以外の引数(updated_at_* / include_archive_flg` +
      `${def.fields.some((f) => f.arg === "tags") ? " / tags" : ""} / response_group などサーバー側の指定を含む)を指定したときは BOARD の API を直接呼び(このとき refresh は無視される)、キャッシュが未同期のときも API を直接呼ぶ。API から取った結果の cache は空になる。${common}`
    );
  }
  return (
    `${head}絞り込みなしなら、キャッシュ(D1)の全件を返し、BOARD は呼ばない(未取得なら items は空)。最新が要るときは refresh=delta か refresh=full を指定する。` +
    `絞り込み(いずれかの引数)を 1 つでも指定すると、キャッシュも refresh も使わず BOARD の API を直接呼ぶ。このとき refresh は無視される。${common}`
  );
}

function getDescription(def: ResourceDef): string {
  const head = `${def.label}を ID で 1 件取得する。${def.summary}。`;
  const common = "結果は {item, cache, _meta?}。item は BOARD の生データ。";
  if (def.mode === "document") {
    return (
      `${head}キャッシュにあればそれを返し(cache に取得時刻)、なければ BOARD から取って保存する。` +
      `refresh=full はキャッシュを使わず BOARD から取り直す。refresh=delta は何もしない(書類には差分更新がない)。${common}`
    );
  }
  const base =
    `${head}キャッシュにあればそれを返し、なければ BOARD から 1 件取って保存する。` +
    `refresh=delta / full を指定すると、先に${def.label}の一覧キャッシュを更新してから読む。${common}`;
  if (def.resource === "projects") {
    return (
      `${base}response_group(${RESPONSE_GROUPS_PROJECTS_GET.join(" / ")})を指定すると、該当する書類を埋め込んだ案件を` +
      "常に BOARD の API から直接取り(キャッシュは使わず、書き込みもしない)、このとき refresh は無視する。"
    );
  }
  return base;
}

// --- ツールの組み立て --------------------------------------------------------------------------

function listTool(def: ResourceDef): ToolDefinition {
  return defineTool({
    name: `list_${def.resource}`,
    title: `${def.label}一覧取得`,
    description: listDescription(def),
    category: "ReadOnly",
    inputSchema: z.strictObject(listShape(def)),
    handler: async (args, ctx) => {
      const a = args as CommonArgs & Record<string, unknown>;
      const repo = (ctx.repositories() as unknown as Record<string, ListRepo>)[def.resource];
      if (!repo) throw new Error(`no repository: ${def.resource}`);
      const r = await repo.list(toListFilter(def, a), readOptions(a), callContextOf(ctx));
      return listOutput(r, a.show_meta);
    },
  });
}

function getTool(def: ResourceDef): ToolDefinition {
  const shape: Record<string, z.ZodType> = {
    id: idArg(`${def.label}の ID(1 以上の整数)`),
    refresh: optEnum(
      REFRESH_CHOICES,
      def.mode === "document"
        ? "none=キャッシュがあればそれを返す(既定)/ full=キャッシュを使わず BOARD から取り直す / delta=何もしない"
        : "none=更新しない(既定)/ delta=差分更新してから読む / full=全件を取り直してから読む(件数が多いと partial になる。続きはもう一度 refresh=full)",
    ),
    show_meta: optBool(SHOW_META_DESC),
  };
  if (def.resource === "projects") {
    shape.response_group = optEnum(
      RESPONSE_GROUPS_PROJECTS_GET,
      "埋め込む書類のグループ: estimate / order / delivery / invoice / receipt / all。指定すると BOARD の API を直接呼び、refresh は無視する",
    );
  }
  return defineTool({
    name: `get_${def.singular}`,
    title: `${def.label}取得`,
    description: getDescription(def),
    category: "ReadOnly",
    inputSchema: z.strictObject(shape),
    handler: async (args, ctx) => {
      const a = args as unknown as CommonArgs & { id: number; response_group?: string };
      const repos = ctx.repositories();
      const call = callContextOf(ctx);
      if (def.resource === "projects" && a.response_group) {
        return getOutput(
          await repos.projects.getWithGroup(a.id, a.response_group, call),
          a.show_meta,
        );
      }
      const repo = repos[def.resource as keyof typeof repos] as {
        get(id: number, o?: ReadOptions, c?: CallContext): Promise<GetReadResult<unknown>>;
      };
      return getOutput(await repo.get(a.id, readOptions(a), call), a.show_meta);
    },
  });
}

interface ListRepo {
  list(filter: object, readOpts: ReadOptions, ctx: CallContext): Promise<ListReadResult<unknown>>;
}

/** list 18 個 → get 22 個の順(表の順)。 */
export const API_TOOLS: readonly ToolDefinition[] = [
  ...API_RESOURCES.filter((d) => d.mode !== "document").map(listTool),
  ...API_RESOURCES.map(getTool),
];
