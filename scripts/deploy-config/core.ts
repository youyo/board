// 下流の設定(board.config.json)と upstream の wrangler.jsonc から、デプロイ用の wrangler 設定(deploy/wrangler.jsonc)を作る純粋なコア。
// rovo の deploy-config-lib に倣う(ADR 0005)。入出力(ファイル・コマンド)は main.ts が担う。ここは文字列とオブジェクトだけを扱う。
// 検証エラーには項目名と理由だけを載せ、入力値は決して載せない。secrets(API キー・トークン)はここでは扱わない。

export type ErrorReason = "missing" | "invalid" | "placeholder" | "unknown";
export interface ConfigError {
  name: string;
  reason: ErrorReason;
}
export type RenderResult =
  | { ok: true; config: Record<string, unknown>; text: string }
  | { ok: false; errors: ConfigError[] };

export interface DeployConfig {
  workerName: string;
  /** 任意。空なら workers.dev で公開する。 */
  customDomain: string;
  d1: { databaseId: string; databaseName: string };
  access: { teamDomain: string; aud: string };
  boardBaseUrl?: string;
  boardUiBaseUrl?: string;
  accountId?: string;
}

/** `deploy:check`(dry-run)用のダミー。実在しない値で、Cloudflare には届かない。 */
export const EXAMPLE_DUMMIES = {
  workerName: "board-example",
  customDomain: "board.example.com",
  databaseId: "00000000-0000-0000-0000-000000000000",
  databaseName: "board-example",
  teamDomain: "example.cloudflareaccess.com",
  aud: "0".repeat(64),
} as const;

const D1_BINDING = "DB";
const PLACEHOLDER = /<[^>]*>/;
const WORKER_NAME = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const DATABASE_NAME = /^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$/;
/** Access アプリケーションの AUD タグは 64 桁の 16 進。 */
const ACCESS_AUD = /^[0-9a-f]{64}$/i;
/** Cloudflare のアカウント ID は 32 桁の 16 進。 */
const ACCOUNT_ID = /^[0-9a-f]{32}$/i;
const HOSTNAME = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i;

/** JSONC(コメント・末尾カンマ)を読む。文字列の中の `//` や `/*` は壊さない。 */
export function parseJsonc(text: string): unknown {
  return JSON.parse(removeTrailingCommas(stripComments(text)));
}

function stripComments(text: string): string {
  let out = "";
  let i = 0;
  while (i < text.length) {
    const c = text[i] as string;
    const next = text[i + 1];
    if (c === '"') {
      const start = i;
      i++;
      while (i < text.length && text[i] !== '"') i += text[i] === "\\" ? 2 : 1;
      i++;
      out += text.slice(start, i);
    } else if (c === "/" && next === "/") {
      while (i < text.length && text[i] !== "\n") i++;
    } else if (c === "/" && next === "*") {
      const end = text.indexOf("*/", i + 2);
      i = end === -1 ? text.length : end + 2;
      out += " ";
    } else {
      out += c;
      i++;
    }
  }
  return out;
}

function removeTrailingCommas(text: string): string {
  let out = "";
  let i = 0;
  while (i < text.length) {
    const c = text[i] as string;
    if (c === '"') {
      const start = i;
      i++;
      while (i < text.length && text[i] !== '"') i += text[i] === "\\" ? 2 : 1;
      i++;
      out += text.slice(start, i);
    } else if (c === ",") {
      let j = i + 1;
      while (j < text.length && /\s/.test(text[j] as string)) j++;
      if (text[j] !== "}" && text[j] !== "]") out += c;
      i++;
    } else {
      out += c;
      i++;
    }
  }
  return out;
}

function isHttpsOrigin(value: string): boolean {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  // https のオリジンだけ。末尾スラッシュ・パス・クエリ・フラグメント・大文字ホストは不可。
  return url.protocol === "https:" && url.origin === value;
}

type Validator = (v: string) => boolean;
type Obj = Record<string, unknown>;

function isObj(v: unknown): v is Obj {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

export interface ValidateOptions {
  /** setup の前提確認用: d1.databaseId が未作成(欠落・空・プレースホルダ)でも許す。生成(render)では使えない。 */
  pendingD1?: boolean;
}

export type ValidateResult =
  | { ok: true; config: DeployConfig }
  | { ok: false; errors: ConfigError[] };

const TOP_KEYS = [
  "workerName",
  "customDomain",
  "d1",
  "access",
  "boardBaseUrl",
  "boardUiBaseUrl",
  "accountId",
];

/** board.config.json(JSON を読んだもの)を検証する。エラーは項目名と理由だけ(値は載せない)。 */
export function validateConfig(raw: unknown, opts: ValidateOptions = {}): ValidateResult {
  if (!isObj(raw)) return { ok: false, errors: [{ name: "board.config.json", reason: "invalid" }] };
  const errors: ConfigError[] = [];

  const read = (
    parent: Obj,
    prefix: string,
    key: string,
    valid: Validator,
    required: boolean,
    pending = false,
  ): string | undefined => {
    const name = prefix ? `${prefix}.${key}` : key;
    const v = parent[key];
    if (v !== undefined && typeof v !== "string") {
      errors.push({ name, reason: "invalid" });
      return undefined;
    }
    const s = (v ?? "").trim();
    if (s === "") {
      if (required && !pending) errors.push({ name, reason: "missing" });
      return undefined;
    }
    if (PLACEHOLDER.test(s)) {
      if (!pending) errors.push({ name, reason: "placeholder" });
    } else if (!valid(s)) errors.push({ name, reason: "invalid" });
    else return s;
    return undefined;
  };

  const section = (key: string, allowed: string[]): Obj | undefined => {
    const v = raw[key];
    if (v === undefined) return {};
    if (!isObj(v)) {
      errors.push({ name: key, reason: "invalid" });
      return undefined;
    }
    for (const k of Object.keys(v)) {
      if (!allowed.includes(k)) errors.push({ name: `${key}.${k}`, reason: "unknown" });
    }
    return v;
  };

  for (const k of Object.keys(raw)) {
    if (!TOP_KEYS.includes(k)) errors.push({ name: k, reason: "unknown" });
  }
  const workerName = read(raw, "", "workerName", (v) => WORKER_NAME.test(v), true);
  // 任意。無い・空なら workers.dev で公開する(ある場合は検証し、不正・プレースホルダはエラー)。
  const customDomain = read(raw, "", "customDomain", (v) => HOSTNAME.test(v), false);
  const d1 = section("d1", ["databaseId", "databaseName"]);
  const databaseId = d1 && read(d1, "d1", "databaseId", (v) => UUID.test(v), true, opts.pendingD1);
  const databaseName = d1 && read(d1, "d1", "databaseName", (v) => DATABASE_NAME.test(v), true);
  const access = section("access", ["teamDomain", "aud"]);
  const teamDomain = access && read(access, "access", "teamDomain", (v) => HOSTNAME.test(v), true);
  const aud = access && read(access, "access", "aud", (v) => ACCESS_AUD.test(v), true);
  const boardBaseUrl = read(raw, "", "boardBaseUrl", isHttpsOrigin, false);
  const boardUiBaseUrl = read(raw, "", "boardUiBaseUrl", isHttpsOrigin, false);
  const accountId = read(raw, "", "accountId", (v) => ACCOUNT_ID.test(v), false);

  if (errors.length > 0 || !workerName || !databaseName || !teamDomain || !aud) {
    return { ok: false, errors };
  }
  if (!databaseId && !opts.pendingD1) return { ok: false, errors };
  return {
    ok: true,
    config: {
      workerName,
      customDomain: customDomain ?? "",
      d1: { databaseId: databaseId ?? "", databaseName },
      access: { teamDomain, aud },
      ...(boardBaseUrl ? { boardBaseUrl } : {}),
      ...(boardUiBaseUrl ? { boardUiBaseUrl } : {}),
      ...(accountId ? { accountId } : {}),
    },
  };
}

export interface ExampleOptions {
  /** true なら customDomain が空でもダミーのドメインを入れる(カスタムドメインの形の検査用)。 */
  withCustomDomain?: boolean;
}

/** board.config.example.json のプレースホルダをダミー値に置き換える(deploy:check 用)。customDomain は空のまま(workers.dev の形)。 */
export function exampleConfig(example: unknown, opts: ExampleOptions = {}): unknown {
  if (!isObj(example)) return example;
  const d = EXAMPLE_DUMMIES;
  const sub = (v: unknown, dummy: string): unknown =>
    typeof v === "string" && PLACEHOLDER.test(v) ? dummy : v;
  const d1 = isObj(example.d1) ? example.d1 : {};
  const access = isObj(example.access) ? example.access : {};
  return {
    ...example,
    workerName: sub(example.workerName, d.workerName),
    customDomain: opts.withCustomDomain
      ? d.customDomain
      : sub(example.customDomain, d.customDomain),
    d1: {
      ...d1,
      databaseId: sub(d1.databaseId, d.databaseId),
      databaseName: sub(d1.databaseName, d.databaseName),
    },
    access: {
      ...access,
      teamDomain: sub(access.teamDomain, d.teamDomain),
      aud: sub(access.aud, d.aud),
    },
  };
}

const GENERATED_HEADER =
  "// 生成物: board.config.json と upstream の wrangler.jsonc から `mise run deploy:config` で作る。手で編集しない(docs/runbook.md)。\n";

export interface RenderOptions {
  /** 生成先ディレクトリからリポジトリ直下への相対パス(deploy/ なら "..")。main と $schema の相対パスに前置する。 */
  relBase: string;
}

function rebase(value: unknown, relBase: string): unknown {
  if (typeof value !== "string" || value === "" || value.startsWith("/")) return value;
  return `${relBase}/${value.replace(/^\.\//, "")}`;
}

export function renderWranglerConfig(
  baseText: string,
  raw: unknown,
  opts: RenderOptions,
): RenderResult {
  const v = validateConfig(raw);
  if (!v.ok) return v;
  const c = v.config;

  let base: Obj | undefined;
  try {
    const parsed = parseJsonc(baseText);
    if (isObj(parsed)) base = parsed;
  } catch {
    // 下で報告する
  }
  const d1 = Array.isArray(base?.d1_databases) ? (base?.d1_databases as Obj[]) : [];
  if (!base || !d1.some((e) => e.binding === D1_BINDING)) {
    return { ok: false, errors: [{ name: "wrangler.jsonc", reason: "invalid" }] };
  }

  // env ブロックは持ち越さない(フラットな設定にする)。routes は作り直す。
  const { env: _env, routes: _routes, ...rest } = base;
  const config: Obj = { ...rest, name: c.workerName };
  if (rest.main !== undefined) config.main = rebase(rest.main, opts.relBase);
  if (rest.$schema !== undefined) config.$schema = rebase(rest.$schema, opts.relBase);
  if (c.accountId) config.account_id = c.accountId;

  config.d1_databases = d1.map((e) =>
    e.binding === D1_BINDING
      ? { ...e, database_name: c.d1.databaseName, database_id: c.d1.databaseId }
      : e,
  );

  const vars = isObj(rest.vars) ? rest.vars : {};
  const { BOARD_UI_BASE_URL: _ui, ...otherVars } = vars;
  config.vars = {
    ...otherVars,
    ACCESS_TEAM_DOMAIN: c.access.teamDomain,
    ACCESS_AUD: c.access.aud,
    ...(c.boardBaseUrl ? { BOARD_BASE_URL: c.boardBaseUrl } : {}),
    ...(c.boardUiBaseUrl ? { BOARD_UI_BASE_URL: c.boardUiBaseUrl } : {}),
  };

  // customDomain があればカスタムドメインだけで公開し、workers.dev は無効にする。
  // なければ workers.dev で公開する(routes は出さない)。どちらも Worker が Access の JWT を検証する。
  // preview_urls(Version URL)は両方で明示的に無効にする(Access を付け忘れやすい公開経路を増やさない。出力も決定的になる)。
  if (c.customDomain) {
    config.routes = [{ pattern: c.customDomain, custom_domain: true }];
    config.workers_dev = false;
  } else {
    config.workers_dev = true;
  }
  config.preview_urls = false;

  return { ok: true, config, text: `${GENERATED_HEADER}${JSON.stringify(config, null, 2)}\n` };
}

/** 標準エラー用。項目名と理由だけ(値は含めない)。 */
export function formatErrors(errors: ConfigError[]): string[] {
  const label: Record<ErrorReason, string> = {
    missing: "未設定",
    invalid: "形式が正しくない",
    placeholder: "プレースホルダ(<...>)のまま",
    unknown: "未知の項目(綴りを確かめる)",
  };
  return errors.map((e) => `${e.name}: ${label[e.reason]}`);
}

/** `<...>` が残る行の番号(1 始まり)。コメント行は除く。deploy が止まる条件。 */
export function findPlaceholderLines(text: string): number[] {
  const lines: number[] = [];
  text.split("\n").forEach((line, i) => {
    if (line.trimStart().startsWith("//")) return;
    if (PLACEHOLDER.test(line)) lines.push(i + 1);
  });
  return lines;
}

/** board.config.json の d1.databaseId だけを書き換えたテキストを返す(setup が D1 の作成後に書き戻す)。 */
export function setD1Id(configText: string, id: string): string {
  if (!UUID.test(id)) throw new Error("d1 の id が uuid ではない");
  const raw = JSON.parse(configText) as unknown;
  if (!isObj(raw)) throw new Error("設定がオブジェクトではない");
  const d1 = isObj(raw.d1) ? raw.d1 : {};
  raw.d1 = { ...d1, databaseId: id };
  return `${JSON.stringify(raw, null, 2)}\n`;
}

/** `wrangler d1 list --json` の出力から、名前で uuid を引く。 */
export function findD1ByName(listJson: string, name: string): string | undefined {
  try {
    const list = JSON.parse(listJson) as unknown;
    if (!Array.isArray(list)) return undefined;
    for (const e of list) {
      if (isObj(e) && e.name === name && typeof e.uuid === "string" && UUID.test(e.uuid)) {
        return e.uuid;
      }
    }
  } catch {
    // 読めなければ見つからない扱い
  }
  return undefined;
}

/** /healthz の 503 の本文から、欠けている項目名と不正な項目名を取り出す(文字列の配列だけ)。 */
export function healthzReport(body: string): { missing: string[]; invalid: string[] } {
  const strings = (v: unknown): string[] =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  try {
    const j = JSON.parse(body) as unknown;
    if (isObj(j)) return { missing: strings(j.missing), invalid: strings(j.invalid) };
  } catch {
    // 読めない
  }
  return { missing: [], invalid: [] };
}
