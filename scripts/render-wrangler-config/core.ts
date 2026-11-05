// リポジトリの wrangler.jsonc と BOARD_* 環境変数から、デプロイ用の wrangler 設定(フラット)を作る純粋なコア。
// logbutler の render-wrangler-config の簡易版(upstream / 下流のモデルは持たない。ロードマップ『デプロイ設定』)。
// 入出力(ファイル・process.env)は main.ts が担う。ここは文字列とオブジェクトだけを扱う。
// 検証エラーには項目名と理由だけを載せ、入力値は決して載せない。secrets(API キー・トークン)はここでは扱わない。

export type ErrorReason = "missing" | "invalid" | "placeholder";
export interface RenderError {
  name: string;
  reason: ErrorReason;
}
export type RenderResult =
  | { ok: true; config: Record<string, unknown>; text: string }
  | { ok: false; errors: RenderError[] };

export const ENV_NAMES = {
  workerName: "BOARD_WORKER_NAME",
  databaseId: "BOARD_D1_DATABASE_ID",
  databaseName: "BOARD_D1_DATABASE_NAME",
  accessTeamDomain: "BOARD_ACCESS_TEAM_DOMAIN",
  accessAud: "BOARD_ACCESS_AUD",
  customDomain: "BOARD_CUSTOM_DOMAIN",
  baseUrl: "BOARD_BASE_URL",
  uiBaseUrl: "BOARD_UI_BASE_URL",
} as const;

/** `deploy:check`(dry-run)用のダミー。実在しない値で、Cloudflare には届かない。 */
export const EXAMPLE_ENV: Record<string, string> = {
  BOARD_WORKER_NAME: "board-example",
  BOARD_D1_DATABASE_ID: "00000000-0000-0000-0000-000000000000",
  BOARD_D1_DATABASE_NAME: "board-example",
  BOARD_ACCESS_TEAM_DOMAIN: "example.cloudflareaccess.com",
  BOARD_ACCESS_AUD: "0000000000000000000000000000000000000000000000000000000000000000",
  BOARD_CUSTOM_DOMAIN: "board.example.com",
  BOARD_UI_BASE_URL: "https://ui.example.com",
};

const D1_BINDING = "DB";
const PLACEHOLDER = /<[^>]*>/;
const WORKER_NAME = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const DATABASE_NAME = /^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$/;
/** Access アプリケーションの AUD タグは 64 桁の 16 進。 */
const ACCESS_AUD = /^[0-9a-f]{64}$/i;
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

export function renderWranglerConfig(
  baseText: string,
  env: Record<string, string | undefined>,
): RenderResult {
  const errors: RenderError[] = [];

  const read = (name: string, valid: Validator, required: boolean): string | undefined => {
    const raw = env[name]?.trim() ?? "";
    if (raw === "") {
      if (required) errors.push({ name, reason: "missing" });
      return undefined;
    }
    if (PLACEHOLDER.test(raw)) errors.push({ name, reason: "placeholder" });
    else if (!valid(raw)) errors.push({ name, reason: "invalid" });
    else return raw;
    return undefined;
  };

  const workerName = read(ENV_NAMES.workerName, (v) => WORKER_NAME.test(v), true);
  const databaseId = read(ENV_NAMES.databaseId, (v) => UUID.test(v), true);
  const databaseName = read(ENV_NAMES.databaseName, (v) => DATABASE_NAME.test(v), true);
  const teamDomain = read(ENV_NAMES.accessTeamDomain, (v) => HOSTNAME.test(v), true);
  const aud = read(ENV_NAMES.accessAud, (v) => ACCESS_AUD.test(v), true);
  const customDomain = read(ENV_NAMES.customDomain, (v) => HOSTNAME.test(v), true);
  const baseUrl = read(ENV_NAMES.baseUrl, isHttpsOrigin, false);
  const uiBaseUrl = read(ENV_NAMES.uiBaseUrl, isHttpsOrigin, false);

  let base: Record<string, unknown> | undefined;
  try {
    const parsed = parseJsonc(baseText);
    if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
      base = parsed as Record<string, unknown>;
    }
  } catch {
    // 下で報告する
  }
  const d1 = Array.isArray(base?.d1_databases)
    ? (base?.d1_databases as Record<string, unknown>[])
    : [];
  if (!base || !d1.some((e) => e.binding === D1_BINDING)) {
    errors.push({ name: "wrangler.jsonc", reason: "invalid" });
  }

  if (errors.length > 0 || !base) return { ok: false, errors };

  // env ブロックは持ち越さない(フラットな設定にする)。routes は作り直す。
  const { env: _env, routes: _routes, ...rest } = base;
  const config: Record<string, unknown> = { ...rest, name: workerName };

  config.d1_databases = d1.map((e) =>
    e.binding === D1_BINDING ? { ...e, database_name: databaseName, database_id: databaseId } : e,
  );

  const vars = (typeof rest.vars === "object" && rest.vars !== null ? rest.vars : {}) as Record<
    string,
    unknown
  >;
  const { BOARD_UI_BASE_URL: _ui, ...otherVars } = vars;
  config.vars = {
    ...otherVars,
    ACCESS_TEAM_DOMAIN: teamDomain,
    ACCESS_AUD: aud,
    ...(baseUrl ? { BOARD_BASE_URL: baseUrl } : {}),
    ...(uiBaseUrl ? { BOARD_UI_BASE_URL: uiBaseUrl } : {}),
  };

  // カスタムドメインだけで公開する。workers.dev は常に無効(Access を迂回できる経路を作らない)。
  config.routes = [{ pattern: customDomain, custom_domain: true }];
  config.workers_dev = false;

  return { ok: true, config, text: `${JSON.stringify(config, null, 2)}\n` };
}

/** 標準エラー用。項目名と理由だけ(値は含めない)。 */
export function formatErrors(errors: RenderError[]): string[] {
  const label: Record<ErrorReason, string> = {
    missing: "未設定",
    invalid: "形式が正しくない",
    placeholder: "プレースホルダ(<...>)のまま",
  };
  return errors.map((e) => `${e.name}: ${label[e.reason]}`);
}
