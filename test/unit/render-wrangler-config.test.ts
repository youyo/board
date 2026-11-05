import { describe, expect, it } from "vitest";
import {
  ENV_NAMES,
  EXAMPLE_ENV,
  formatErrors,
  parseJsonc,
  renderWranglerConfig,
} from "../../scripts/render-wrangler-config/core";

type Config = Record<string, unknown> & {
  vars: Record<string, string>;
  d1_databases: Array<Record<string, string>>;
  routes?: unknown[];
};

// リポジトリの wrangler.jsonc に近い入力(コメント・末尾カンマを含む)。
const BASE = `{
  // コメント
  "$schema": "node_modules/wrangler/config-schema.json",
  "name": "board",
  "main": "src/index.ts",
  "compatibility_date": "2026-10-13",
  "compatibility_flags": [],
  "workers_dev": false,
  "observability": { "enabled": true },
  "d1_databases": [
    { "binding": "DB", "database_name": "board", "database_id": "00000000-0000-0000-0000-000000000000" },
  ],
  "vars": {
    "ACCESS_TEAM_DOMAIN": "<team>.cloudflareaccess.com", // 末尾コメント
    "ACCESS_AUD": "<access-application-aud>",
    "BOARD_BASE_URL": "https://api.the-board.jp",
    "EXTRA": "http://not-a-comment.example/*x*/",
  },
}`;

const VALID = {
  BOARD_WORKER_NAME: "my-board",
  BOARD_D1_DATABASE_ID: "0123abcd-4567-89ab-cdef-0123456789ab",
  BOARD_D1_DATABASE_NAME: "my-board-cache",
  BOARD_ACCESS_TEAM_DOMAIN: "myteam.cloudflareaccess.com",
  BOARD_ACCESS_AUD: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // gitleaks:allow テスト用の合成値
  BOARD_CUSTOM_DOMAIN: "board.example.org",
};

function ok(env: Record<string, string | undefined>) {
  const r = renderWranglerConfig(BASE, env);
  if (!r.ok) throw new Error(`expected ok: ${JSON.stringify(r.errors)}`);
  return r;
}

function errorsOf(env: Record<string, string | undefined>) {
  const r = renderWranglerConfig(BASE, env);
  if (r.ok) throw new Error("expected errors");
  return r.errors;
}

describe("parseJsonc", () => {
  it("コメントと末尾カンマを許し、文字列内の // や /* は壊さない", () => {
    const v = parseJsonc('{ "a": "http://x/*y*/", // c\n "b": [1,2,], /* d */ }');
    expect(v).toEqual({ a: "http://x/*y*/", b: [1, 2] });
  });
});

describe("renderWranglerConfig", () => {
  it("Worker 名・D1・Access・カスタムドメインを差し替え、ほかは引き継ぐ", () => {
    const config = ok(VALID).config as Config;
    expect(config.name).toBe("my-board");
    expect(config.d1_databases).toEqual([
      {
        binding: "DB",
        database_name: "my-board-cache",
        database_id: "0123abcd-4567-89ab-cdef-0123456789ab",
      },
    ]);
    expect(config.vars.ACCESS_TEAM_DOMAIN).toBe("myteam.cloudflareaccess.com");
    expect(config.vars.ACCESS_AUD).toBe(VALID.BOARD_ACCESS_AUD);
    expect(config.vars.EXTRA).toBe("http://not-a-comment.example/*x*/");
    expect(config.routes).toEqual([{ pattern: "board.example.org", custom_domain: true }]);
    expect(config.main).toBe("src/index.ts");
    expect(config.compatibility_date).toBe("2026-10-13");
    expect(config.observability).toEqual({ enabled: true });
  });

  it("workers_dev は常に false(Access を迂回できる経路を作らない)", () => {
    expect(ok(VALID).config.workers_dev).toBe(false);
  });

  it("BOARD_BASE_URL は省略すると wrangler.jsonc の値を残し、指定すれば差し替える", () => {
    expect((ok(VALID).config as Config).vars.BOARD_BASE_URL).toBe("https://api.the-board.jp");
    const c = ok({ ...VALID, BOARD_BASE_URL: "https://api.example.org" }).config as Config;
    expect(c.vars.BOARD_BASE_URL).toBe("https://api.example.org");
  });

  it("BOARD_UI_BASE_URL は指定したときだけ vars に入る", () => {
    expect((ok(VALID).config as Config).vars.BOARD_UI_BASE_URL).toBeUndefined();
    const c = ok({ ...VALID, BOARD_UI_BASE_URL: "https://ui.example.org" }).config as Config;
    expect(c.vars.BOARD_UI_BASE_URL).toBe("https://ui.example.org");
  });

  it("出力のテキストは JSON として読み戻せる", () => {
    const r = ok(VALID);
    expect(JSON.parse(r.text)).toEqual(r.config);
  });

  it("secrets(API キー・トークン)は設定に載せない", () => {
    const r = ok({ ...VALID, BOARD_API_KEY: "k-secret", BOARD_API_TOKEN: "t-secret" });
    expect(r.text).not.toContain("secret");
    expect(r.text).not.toContain("BOARD_API_KEY");
  });

  it("EXAMPLE_ENV(deploy:check 用)で生成できる", () => {
    expect(renderWranglerConfig(BASE, EXAMPLE_ENV).ok).toBe(true);
  });
});

describe("検証エラー(項目名と理由だけ。値は載せない)", () => {
  const REQUIRED = [
    ENV_NAMES.workerName,
    ENV_NAMES.databaseId,
    ENV_NAMES.databaseName,
    ENV_NAMES.accessTeamDomain,
    ENV_NAMES.accessAud,
    ENV_NAMES.customDomain,
  ];

  it.each(REQUIRED)("%s が無ければ missing", (name) => {
    const { [name]: _omit, ...rest } = VALID as Record<string, string>;
    expect(errorsOf(rest)).toEqual([{ name, reason: "missing" }]);
  });

  it("空文字・空白だけは未設定", () => {
    expect(errorsOf({ ...VALID, BOARD_WORKER_NAME: "  " })).toEqual([
      { name: "BOARD_WORKER_NAME", reason: "missing" },
    ]);
  });

  it("プレースホルダ(<...>)のままなら placeholder", () => {
    expect(
      errorsOf({
        ...VALID,
        BOARD_ACCESS_AUD: "<access-application-aud>",
        BOARD_ACCESS_TEAM_DOMAIN: "<team>.cloudflareaccess.com",
      }),
    ).toEqual([
      { name: "BOARD_ACCESS_TEAM_DOMAIN", reason: "placeholder" },
      { name: "BOARD_ACCESS_AUD", reason: "placeholder" },
    ]);
  });

  it.each([
    ["BOARD_WORKER_NAME", "My_Board"],
    ["BOARD_D1_DATABASE_ID", "not-a-uuid"],
    ["BOARD_D1_DATABASE_NAME", "has space"],
    ["BOARD_ACCESS_TEAM_DOMAIN", "https://myteam.cloudflareaccess.com"],
    ["BOARD_ACCESS_AUD", "too-short"],
    ["BOARD_CUSTOM_DOMAIN", "https://board.example.org"],
    ["BOARD_BASE_URL", "http://api.example.org"],
    ["BOARD_BASE_URL", "https://api.example.org/v1"],
    ["BOARD_UI_BASE_URL", "http://ui.example.org"],
    ["BOARD_UI_BASE_URL", "ui.example.org"],
  ])("%s=%s は invalid", (name, value) => {
    expect(errorsOf({ ...VALID, [name]: value })).toEqual([{ name, reason: "invalid" }]);
  });

  it("formatErrors は項目名と理由だけを返す", () => {
    const lines = formatErrors(errorsOf({ ...VALID, BOARD_D1_DATABASE_ID: "SECRET-LOOKING" }));
    expect(lines).toHaveLength(1);
    expect(lines[0]).toContain("BOARD_D1_DATABASE_ID");
    expect(lines.join("")).not.toContain("SECRET");
  });

  it("wrangler.jsonc が読めない・D1 の DB 束縛が無いなら wrangler.jsonc の invalid", () => {
    expect(renderWranglerConfig("{ broken", VALID)).toEqual({
      ok: false,
      errors: [{ name: "wrangler.jsonc", reason: "invalid" }],
    });
    const noD1 = BASE.replace(/"d1_databases"[\s\S]*?\],\n/, "");
    expect(renderWranglerConfig(noD1, VALID)).toEqual({
      ok: false,
      errors: [{ name: "wrangler.jsonc", reason: "invalid" }],
    });
  });
});
