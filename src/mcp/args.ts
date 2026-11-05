/**
 * ツール共通の入力スキーマ(zod)のビルダー。logbutler の `args.ts` に揃える。
 *
 * 整数は `.refine` で検証するので、公開される JSON Schema は `{ type: "number", description }` だけになる。
 * ツールの入力は必ず `z.strictObject` にする(未知の引数は argument_error)。
 */
import { z } from "zod";

const isInt = (n: number) => Number.isInteger(n);

/**
 * 入力の上限。上限のない入力は、巨大な文字列・配列を BOARD のクエリや D1 の検索に流してしまうので、スキーマで先に断る(argument_error)。
 * - 文字列 1,000 文字: BOARD の名前・タグ・ステータス・日付などの絞り込み値は数十文字に収まる。その 20 倍以上の余裕を見た値。
 * - 配列 100 件: 絞り込みに使う ID・タグの現実的な数(22 リソースの cache_clear も収まる)の数倍。URL(クエリ)が過大にならない範囲。
 */
export const MAX_STRING_LENGTH = 1000;
export const MAX_ARRAY_ITEMS = 100;

/** 長さの上限つきの文字列(スキーマの部品)。 */
export const boundedString = () => z.string().max(MAX_STRING_LENGTH);

/** 任意の文字列。 */
export const optStr = (description: string) => boundedString().describe(description).optional();
/** 任意の整数(JSON Schema 上は number)。 */
export const optInt = (description: string) =>
  z.number().refine(isInt, "must be an integer").describe(description).optional();
/** 任意の真偽値。 */
export const optBool = (description: string) => z.boolean().describe(description).optional();
/** 任意の文字列の配列。 */
export const optStrArray = (description: string) =>
  z.array(boundedString()).max(MAX_ARRAY_ITEMS).describe(description).optional();
/** 任意の整数の配列。 */
export const optIntArray = (description: string) =>
  z
    .array(z.number().refine(isInt, "must be an integer"))
    .max(MAX_ARRAY_ITEMS)
    .describe(description)
    .optional();
/** 任意の列挙値。 */
export const optEnum = <const T extends readonly [string, ...string[]]>(
  values: T,
  description: string,
) => z.enum(values).describe(description).optional();

/** 必須の ID(1 以上の整数。JSON Schema 上は number)。 */
export const idArg = (description: string) =>
  z
    .number()
    .refine((n) => isInt(n) && n >= 1, "must be a positive integer")
    .describe(description);

/**
 * 任意の ID。0 は『指定なし』(Go の getIntArg と同じ)で、負の数・小数は argument_error。
 * 0 を取り除くのはハンドラの責務(`idOrUndefined`)。JSON Schema 上は number。
 */
export const optId = (description: string) =>
  z
    .number()
    .refine((n) => isInt(n) && n >= 0, "must be a positive integer (0 or omitted = not specified)")
    .describe(description)
    .optional();

/** `optId` の値を検索の条件に直す。0 は『指定なし』なので undefined。 */
export const idOrUndefined = (n: number | undefined): number | undefined =>
  n === undefined || n === 0 ? undefined : n;

/** 0 以上の整数(JSON Schema 上は number)。 */
export const optNonNegativeInt = (description: string) =>
  z
    .number()
    .refine((n) => isInt(n) && n >= 0, "must be an integer of 0 or greater")
    .describe(description)
    .optional();

export const REFRESH_CHOICES = ["none", "delta", "full"] as const;
