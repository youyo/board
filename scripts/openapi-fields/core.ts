// OpenAPI 仕様(BOARD 公式 v1.9.0)から、22 リソースの応答キーと一覧のクエリ名を取り出す純粋なコア(issue #28)。
// 入出力は main.ts。ここは JSON → 生成物の文字列だけ(Node 24 の型ストリップでも tsc でも読める書き方にする)。

export const OPENAPI_SOURCE_URL = "https://developers.the-board.jp/doc/board_openapi.json";

/** リソース名 → 仕様のパス(`servers` の /v1 を除く)。list が無いもの(書類)は null。 */
export const RESOURCE_PATHS: Readonly<Record<string, { list: string | null; get: string | null }>> =
  {
    clients: { list: "/clients", get: "/clients/{id}" },
    client_branches: { list: "/client_branches", get: "/client_branches/{id}" },
    contacts: { list: "/contacts", get: "/contacts/{id}" },
    projects: { list: "/projects", get: "/projects/{id}" },
    project_costs: { list: "/project_costs", get: "/project_costs/{id}" },
    estimates: { list: null, get: "/documents/estimates/{id}" },
    invoices: { list: "/invoices", get: null },
    orders: { list: null, get: "/documents/orders/{id}" },
    deliveries: { list: null, get: "/documents/deliveries/{id}" },
    receipts: { list: null, get: "/documents/receipts/{id}" },
    vendors: { list: "/payees", get: "/payees/{id}" },
    vendor_branches: { list: "/payee_branches", get: "/payee_branches/{id}" },
    vendor_contacts: { list: "/payee_contacts", get: "/payee_contacts/{id}" },
    purchase_orders: { list: "/expenditures", get: "/expenditures/{id}" },
    payments: { list: "/expenditure_payments", get: null },
    users: { list: "/users", get: null },
    groups: { list: "/groups", get: null },
    payment_terms: { list: "/payment_terms", get: null },
    project_types: { list: "/project_types", get: null },
    purchase_types: { list: "/expenditure_types", get: null },
    accounting_types: { list: "/accounting_types", get: null },
    document_send_channels: { list: "/document_send_channels", get: null },
  };

export interface ExtractedResource {
  listPath: string | null;
  getPath: string | null;
  /** 一覧と単体取得の応答(トップレベル)のキーの和集合。response_group で増えるキーを含む。 */
  responseKeys: string[];
  /** 入れ子のオブジェクト / オブジェクトの配列のキー(トップレベルのキー名 → その中のキー)。和集合。 */
  nestedKeys: Record<string, string[]>;
  /** 一覧のクエリ名(`x[]` の形も仕様の parameters の名前どおり)。 */
  listQuery: string[];
}

export interface Extracted {
  version: string;
  resources: Record<string, ExtractedResource>;
}

type Json = Record<string, unknown>;
const isObj = (v: unknown): v is Json => typeof v === "object" && v !== null && !Array.isArray(v);

export function extract(spec: unknown): Extracted {
  if (!isObj(spec)) throw new Error("OpenAPI: root is not an object");
  const root = spec;
  const info = root.info;
  const version = isObj(info) && typeof info.version === "string" ? info.version : "";
  if (!version) throw new Error("OpenAPI: info.version is missing");
  const paths = root.paths;
  if (!isObj(paths)) throw new Error("OpenAPI: paths is missing");

  const deref = (node: unknown): unknown => {
    let cur = node;
    for (let i = 0; i < 20 && isObj(cur) && typeof cur.$ref === "string"; i++) {
      const ref: string = cur.$ref;
      if (!ref.startsWith("#/")) throw new Error(`OpenAPI: unsupported $ref ${ref}`);
      let next: unknown = root;
      for (const seg of ref.slice(2).split("/")) {
        if (!isObj(next)) throw new Error(`OpenAPI: broken $ref ${ref}`);
        next = next[seg.replace(/~1/g, "/").replace(/~0/g, "~")];
      }
      cur = next;
    }
    return cur;
  };

  /** スキーマの properties を allOf / anyOf / oneOf と配列の items をたどって集める(キー名 → スキーマ)。 */
  const properties = (schema: unknown, depth = 0): Map<string, unknown> => {
    const out = new Map<string, unknown>();
    const s = deref(schema);
    if (!isObj(s) || depth > 10) return out;
    if (s.type === "array" || "items" in s) {
      for (const [k, v] of properties(s.items, depth + 1)) out.set(k, v);
      return out;
    }
    for (const comb of ["allOf", "anyOf", "oneOf"]) {
      const list = s[comb];
      if (Array.isArray(list)) {
        for (const sub of list) for (const [k, v] of properties(sub, depth + 1)) out.set(k, v);
      }
    }
    if (isObj(s.properties)) for (const [k, v] of Object.entries(s.properties)) out.set(k, v);
    return out;
  };

  const operation = (path: string): Json => {
    const item = paths[path];
    if (!isObj(item) || !isObj(item.get)) throw new Error(`OpenAPI: GET ${path} is missing`);
    return item.get;
  };

  const responseSchema = (op: Json, path: string): unknown => {
    const responses = op.responses;
    const ok = isObj(responses) ? deref(responses["200"]) : undefined;
    const content = isObj(ok) ? ok.content : undefined;
    const json = isObj(content) ? content["application/json"] : undefined;
    if (!isObj(json) || json.schema === undefined) {
      throw new Error(`OpenAPI: GET ${path} has no 200 application/json schema`);
    }
    return json.schema;
  };

  const resources: Record<string, ExtractedResource> = {};
  for (const [name, p] of Object.entries(RESOURCE_PATHS)) {
    const keys = new Set<string>();
    const nested = new Map<string, Set<string>>();
    for (const path of [p.list, p.get]) {
      if (path === null) continue;
      for (const [key, sub] of properties(responseSchema(operation(path), path))) {
        keys.add(key);
        const inner = properties(sub);
        if (inner.size > 0) {
          const set = nested.get(key) ?? new Set<string>();
          for (const k of inner.keys()) set.add(k);
          nested.set(key, set);
        }
      }
    }
    let listQuery: string[] = [];
    if (p.list !== null) {
      const params = operation(p.list).parameters;
      if (!Array.isArray(params)) throw new Error(`OpenAPI: GET ${p.list} has no parameters`);
      listQuery = params
        .map(deref)
        .filter((x): x is Json => isObj(x) && x.in === "query" && typeof x.name === "string")
        .map((x) => x.name as string);
    }
    resources[name] = {
      listPath: p.list,
      getPath: p.get,
      responseKeys: [...keys],
      nestedKeys: Object.fromEntries([...nested].map(([k, v]) => [k, [...v]])),
      listQuery,
    };
  }
  return { version, resources };
}

const q = (s: string): string => JSON.stringify(s);
const LINE_WIDTH = 100;
/** 識別子として書けるキーは引用符を外す(biome の quoteProperties: asNeeded に合わせる)。 */
const key = (k: string): string => (/^[A-Za-z_$][\w$]*$/.test(k) ? k : q(k));
/**
 * `<head>[...],` を 1 行に収まるなら 1 行、収まらなければ 1 要素ずつの複数行で書く(biome の整形結果と同じにして、生成物が lint で再整形されないようにする)。
 */
const arrayLines = (indent: string, head: string, xs: readonly string[]): string[] => {
  const one = `${indent}${head}[${xs.map(q).join(", ")}],`;
  if (one.length <= LINE_WIDTH) return [one];
  return [`${indent}${head}[`, ...xs.map((x) => `${indent}  ${q(x)},`), `${indent}],`];
};

const HEADER = (version: string, file: string): string =>
  `// 自動生成: mise run openapi:fields -- <board_openapi.json> で作る。手で編集しない(${file})。\n` +
  `// 元: BOARD 公式の OpenAPI 仕様 v${version}(${OPENAPI_SOURCE_URL})。仕様の JSON 自体はリポジトリに置かない。\n`;

/** test/helpers/openapi-fields.ts の中身。 */
export function renderTestFields(e: Extracted): string {
  const lines: string[] = [];
  lines.push(HEADER(e.version, "scripts/openapi-fields/"));
  lines.push(`/** 仕様のバージョン(info.version)。 */`);
  lines.push(`export const OPENAPI_SPEC_VERSION = ${q(e.version)};`);
  lines.push(`/** 仕様の取得元。 */`);
  lines.push(`export const OPENAPI_SPEC_URL = ${q(OPENAPI_SOURCE_URL)};\n`);
  lines.push(`/**`);
  lines.push(
    ` * 22 リソースの応答キー(一覧と単体取得の和集合。response_group で増えるキーを含む)と、一覧のクエリ名。`,
  );
  lines.push(
    ` * nestedKeys は入れ子のオブジェクトの中のキー、listQuery は一覧の parameters の名前(\`x[]\` の形も仕様どおり)。`,
  );
  lines.push(` */`);
  lines.push(`export const OPENAPI_RESOURCES = {`);
  for (const [name, r] of Object.entries(e.resources)) {
    lines.push(`  ${name}: {`);
    lines.push(`    listPath: ${r.listPath === null ? "null" : q(r.listPath)},`);
    lines.push(`    getPath: ${r.getPath === null ? "null" : q(r.getPath)},`);
    lines.push(...arrayLines("    ", "responseKeys: ", r.responseKeys));
    const nested = Object.entries(r.nestedKeys);
    if (nested.length === 0) lines.push(`    nestedKeys: {},`);
    else {
      lines.push(`    nestedKeys: {`);
      for (const [k, v] of nested) lines.push(...arrayLines("      ", `${key(k)}: `, v));
      lines.push(`    },`);
    }
    lines.push(...arrayLines("    ", "listQuery: ", r.listQuery));
    lines.push(`  },`);
  }
  lines.push(`} as const;\n`);
  lines.push(`export type OpenApiResourceName = keyof typeof OPENAPI_RESOURCES;\n`);
  return `${lines.join("\n")}`;
}

/** 共通フィルタ(updated_at_*・include_archive_flg)を一覧が受けるか。 */
export const SUPPORT_FIELDS = [
  ["updatedAtGteq", "updated_at_gteq"],
  ["updatedAtLteq", "updated_at_lteq"],
  ["includeArchiveFlg", "include_archive_flg"],
] as const;

/** src/board/openapi-support.ts の中身。Worker が実行時に使う、仕様から導いた小さな表。 */
export function renderSupport(e: Extracted): string {
  const lines: string[] = [];
  lines.push(HEADER(e.version, "scripts/openapi-fields/"));
  lines.push(`export interface ListFilterSupport {`);
  for (const [prop, name] of SUPPORT_FIELDS) {
    lines.push(`  /** 一覧のクエリに ${name} があるか */`);
    lines.push(`  ${prop}: boolean;`);
  }
  lines.push(`}\n`);
  lines.push(
    `/** リソースごとの、一覧が受ける共通フィルタ。仕様に無いものは API に送らず、ツールの引数にも出さない。 */`,
  );
  lines.push(`export const LIST_FILTER_SUPPORT: Readonly<Record<string, ListFilterSupport>> = {`);
  for (const [name, r] of Object.entries(e.resources)) {
    if (r.listPath === null) continue;
    const flags = SUPPORT_FIELDS.map(([prop, qn]) => `${prop}: ${r.listQuery.includes(qn)}`);
    const one = `  ${name}: { ${flags.join(", ")} },`;
    if (one.length <= LINE_WIDTH) lines.push(one);
    else lines.push(`  ${name}: {`, ...flags.map((f) => `    ${f},`), `  },`);
  }
  lines.push(`};\n`);
  return lines.join("\n");
}
