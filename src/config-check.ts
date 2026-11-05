/**
 * 起動時の設定チェック(logbutler の ADR 0010 と同じ)。必須の vars / secrets / D1 が欠けている、
 * またはプレースホルダ(`<...>`)のまま・形式が違う場合に、項目名だけを返す。値は結果にも例外にも載せない。
 */
export interface ConfigCheckResult {
  ok: boolean;
  /** 未設定(未定義・空・D1 でない)の項目名。 */
  missing: string[];
  /** 設定されているが使えない(プレースホルダのまま・形式違い)項目名。 */
  invalid: string[];
}

const PLACEHOLDER = /<[^>]*>/;
const HOST = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+(:\d+)?$/i;
const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]"]);

function isBlank(value: unknown): boolean {
  return typeof value !== "string" || value.trim() === "";
}

function isD1Database(value: unknown): boolean {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as { prepare?: unknown }).prepare === "function"
  );
}

/** https のオリジンだけ(UI の URL 用。http は loopback でも不可)。 */
function isHttpsOrigin(value: string): boolean {
  return isValidBaseUrl(value) && new URL(value).protocol === "https:";
}

/** https のオリジンだけ(ローカル開発の loopback は http も可)。パス(末尾 / を除く)・クエリ・フラグメントは不可。 */
function isValidBaseUrl(value: string): boolean {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return false;
  }
  const schemeOk =
    url.protocol === "https:" || (url.protocol === "http:" && LOOPBACK_HOSTS.has(url.hostname));
  return schemeOk && url.pathname === "/" && url.search === "" && url.hash === "";
}

export function checkConfig(env: Record<string, unknown>): ConfigCheckResult {
  const missing: string[] = [];
  const invalid: string[] = [];

  if (!isD1Database(env.DB)) missing.push("DB");

  const text = (name: string, valid?: (v: string) => boolean) => {
    const value = env[name];
    if (isBlank(value)) {
      missing.push(name);
      return;
    }
    const v = (value as string).trim();
    if (PLACEHOLDER.test(v) || (valid && !valid(v))) invalid.push(name);
  };
  text("ACCESS_TEAM_DOMAIN", (v) => HOST.test(v));
  text("ACCESS_AUD");
  text("BOARD_BASE_URL", isValidBaseUrl);
  // 任意。未設定・空は問題にしない。設定されているなら https のオリジンであること。
  if (!isBlank(env.BOARD_UI_BASE_URL)) text("BOARD_UI_BASE_URL", isHttpsOrigin);
  text("BOARD_API_KEY");
  text("BOARD_API_TOKEN");

  return { ok: missing.length === 0 && invalid.length === 0, missing, invalid };
}
