/**
 * BOARD API のクエリ文字列の組み立て(Go 版 `internal/boardapi/query.go` の移植)。
 *
 * - ゼロ値・空文字・undefined は送らない(Go の「ゼロ値は送らない」と同じ)。
 * - `<field>_eq` / `<field>_cont` / `<field>_gteq` / `<field>_lteq` は Ransack の記法。
 * - 配列は `<field>_in[]` / `tags[]` を繰り返して送る。
 * - 真偽値の `*_flg` は `0` / `1`。
 * - `encode()` は Go の `url.Values.Encode()` と同じ形式(キーを辞書順、同じキーの値は追加順、`QueryEscape`)。
 */

import { ArgumentError } from "../errors";

/** Go の url.QueryEscape: `A-Za-z0-9-_.~` 以外は %XX、空白は `+`。 */
function queryEscape(s: string): string {
  let encoded: string;
  try {
    encoded = encodeURIComponent(s);
  } catch {
    throw new ArgumentError("Query parameter contains invalid characters (unpaired surrogate).");
  }
  return encoded
    .replace(/[!'()*]/g, (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`)
    .replace(/%20/g, "+");
}

export class QueryBuilder {
  private readonly values = new Map<string, string[]>();

  private setValue(key: string, value: string): this {
    this.values.set(key, [value]);
    return this;
  }

  private addValue(key: string, value: string): this {
    const list = this.values.get(key);
    if (list) list.push(value);
    else this.values.set(key, [value]);
    return this;
  }

  /** `page` と `per_page`。0 以下は送らない。 */
  page(page: number | undefined, perPage: number | undefined): this {
    if (page !== undefined && page > 0) this.setValue("page", String(page));
    if (perPage !== undefined && perPage > 0) this.setValue("per_page", String(perPage));
    return this;
  }

  /** `<field>_eq=<value>`(空は送らない)。 */
  strEq(field: string, value: string | undefined): this {
    return value ? this.setValue(`${field}_eq`, value) : this;
  }

  /** `<field>_cont=<value>`(部分一致。空は送らない)。 */
  strCont(field: string, value: string | undefined): this {
    return value ? this.setValue(`${field}_cont`, value) : this;
  }

  /** `<field>_eq=<value>`(0 と undefined は送らない)。 */
  intEq(field: string, value: number | undefined): this {
    return value ? this.setValue(`${field}_eq`, String(value)) : this;
  }

  /** `<field>_in[]=<v>` の繰り返し。 */
  intIn(field: string, values: readonly number[] | undefined): this {
    for (const v of values ?? []) this.addValue(`${field}_in[]`, String(v));
    return this;
  }

  /** `<field>_in[]=<v>` の繰り返し(空要素は飛ばす)。 */
  strIn(field: string, values: readonly string[] | undefined): this {
    for (const v of values ?? []) if (v) this.addValue(`${field}_in[]`, v);
    return this;
  }

  /** `<field>_gteq=<value>`(以上)。`yyyy-MM-dd` か `yyyy-MM-dd HH:mm:ss`。 */
  dateGteq(field: string, value: string | undefined): this {
    return value ? this.setValue(`${field}_gteq`, value) : this;
  }

  /** `<field>_lteq=<value>`(以下)。 */
  dateLteq(field: string, value: string | undefined): this {
    return value ? this.setValue(`${field}_lteq`, value) : this;
  }

  /** `<field>=0|1`。undefined は「絞り込まない」ので送らない。 */
  flg01(field: string, value: boolean | undefined): this {
    return value === undefined || value === null ? this : this.setValue(field, value ? "1" : "0");
  }

  /** `tags[]=<v>` の繰り返し(空要素は飛ばす)。 */
  tags(values: readonly string[] | undefined): this {
    for (const v of values ?? []) if (v) this.addValue("tags[]", v);
    return this;
  }

  /** `response_group=<value>`。 */
  responseGroup(value: string | undefined): this {
    return value ? this.setValue("response_group", value) : this;
  }

  /** 任意のキーと値(空は送らない)。 */
  set(key: string, value: string | undefined): this {
    return value ? this.setValue(key, value) : this;
  }

  encode(): string {
    const keys = [...this.values.keys()].sort();
    const parts: string[] = [];
    for (const key of keys) {
      for (const v of this.values.get(key) ?? []) {
        parts.push(`${queryEscape(key)}=${queryEscape(v)}`);
      }
    }
    return parts.join("&");
  }
}
