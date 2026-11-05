import { describe, expect, it } from "vitest";
import exampleText from "../../board.config.example.json?raw";
import {
  type ConfigError,
  EXAMPLE_DUMMIES,
  exampleConfig,
  findD1ByName,
  findPlaceholderLines,
  formatErrors,
  healthzReport,
  parseJsonc,
  renderWranglerConfig,
  setD1Id,
  validateConfig,
} from "../../scripts/deploy-config/core";
import upstreamWrangler from "../../wrangler.jsonc?raw";

type Rendered = Record<string, unknown> & {
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

const VALID_AUD = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"; // gitleaks:allow テスト用の合成値

function valid(): Record<string, unknown> {
  return {
    workerName: "my-board",
    customDomain: "board.example.org",
    d1: { databaseId: "0123abcd-4567-89ab-cdef-0123456789ab", databaseName: "my-board-cache" },
    access: { teamDomain: "myteam.cloudflareaccess.com", aud: VALID_AUD },
  };
}

function ok(raw: unknown = valid(), relBase = "..") {
  const r = renderWranglerConfig(BASE, raw, { relBase });
  if (!r.ok) throw new Error(`expected ok: ${JSON.stringify(r.errors)}`);
  return r;
}

function errorsOf(raw: unknown, opts?: { pendingD1?: boolean }): ConfigError[] {
  const r = validateConfig(raw, opts);
  if (r.ok) throw new Error("expected errors");
  return r.errors;
}

/** valid() の d1 / access の下の 1 項目を差し替える・消す。 */
function patched(path: string, value: unknown): Record<string, unknown> {
  const raw = valid() as Record<string, Record<string, unknown> | unknown>;
  const [a, b] = path.split(".");
  if (b === undefined) {
    if (value === undefined) delete raw[a as string];
    else raw[a as string] = value;
  } else {
    const parent = raw[a as string] as Record<string, unknown>;
    if (value === undefined) delete parent[b];
    else parent[b] = value;
  }
  return raw;
}

describe("parseJsonc", () => {
  it("コメントと末尾カンマを許し、文字列内の // や /* は壊さない", () => {
    const v = parseJsonc('{ "a": "http://x/*y*/", // c\n "b": [1,2,], /* d */ }');
    expect(v).toEqual({ a: "http://x/*y*/", b: [1, 2] });
  });
});

describe("renderWranglerConfig", () => {
  it("Worker 名・D1・Access・カスタムドメインを差し替え、ほかは引き継ぐ", () => {
    const config = parseJsonc(ok().text) as Rendered;
    expect(config.name).toBe("my-board");
    expect(config.d1_databases).toEqual([
      {
        binding: "DB",
        database_name: "my-board-cache",
        database_id: "0123abcd-4567-89ab-cdef-0123456789ab",
      },
    ]);
    expect(config.vars.ACCESS_TEAM_DOMAIN).toBe("myteam.cloudflareaccess.com");
    expect(config.vars.ACCESS_AUD).toBe(VALID_AUD);
    expect(config.vars.EXTRA).toBe("http://not-a-comment.example/*x*/");
    expect(config.routes).toEqual([{ pattern: "board.example.org", custom_domain: true }]);
    expect(config.compatibility_date).toBe("2026-10-13");
    expect(config.observability).toEqual({ enabled: true });
  });

  it("main と $schema の相対パスを、生成先のディレクトリから見たパスに直す", () => {
    const one = parseJsonc(ok(valid(), "..").text) as Rendered;
    expect(one.main).toBe("../src/index.ts");
    expect(one.$schema).toBe("../node_modules/wrangler/config-schema.json");
    const two = parseJsonc(ok(valid(), "../..").text) as Rendered;
    expect(two.main).toBe("../../src/index.ts");
  });

  it("絶対パスの main はそのまま", () => {
    const base = BASE.replace('"src/index.ts"', '"/abs/index.ts"');
    const r = renderWranglerConfig(base, valid(), { relBase: ".." });
    expect(r.ok && (parseJsonc(r.text) as Rendered).main).toBe("/abs/index.ts");
  });

  it("workers_dev は常に false(Access を迂回できる経路を作らない)", () => {
    const base = BASE.replace('"workers_dev": false', '"workers_dev": true');
    const r = renderWranglerConfig(base, valid(), { relBase: ".." });
    expect(r.ok && (parseJsonc(r.text) as Rendered).workers_dev).toBe(false);
  });

  it("boardBaseUrl は省略すると wrangler.jsonc の値を残し、指定すれば差し替える", () => {
    expect((parseJsonc(ok().text) as Rendered).vars.BOARD_BASE_URL).toBe(
      "https://api.the-board.jp",
    );
    const c = parseJsonc(ok({ ...valid(), boardBaseUrl: "https://api.example.org" }).text);
    expect((c as Rendered).vars.BOARD_BASE_URL).toBe("https://api.example.org");
  });

  it("boardUiBaseUrl は指定したときだけ vars に入る", () => {
    expect((parseJsonc(ok().text) as Rendered).vars.BOARD_UI_BASE_URL).toBeUndefined();
    const c = parseJsonc(ok({ ...valid(), boardUiBaseUrl: "https://ui.example.org" }).text);
    expect((c as Rendered).vars.BOARD_UI_BASE_URL).toBe("https://ui.example.org");
  });

  it("accountId は指定したときだけ account_id に入る(空文字は未指定)", () => {
    expect((parseJsonc(ok().text) as Rendered).account_id).toBeUndefined();
    expect(
      (parseJsonc(ok({ ...valid(), accountId: "" }).text) as Rendered).account_id,
    ).toBeUndefined();
    const id = "0123456789abcdef0123456789abcdef"; // gitleaks:allow テスト用の合成値
    expect((parseJsonc(ok({ ...valid(), accountId: id }).text) as Rendered).account_id).toBe(id);
  });

  it("先頭に生成物であることを示すコメントがあり、末尾は改行。同じ入力なら同じ出力", () => {
    const a = ok().text;
    expect(a.startsWith("//")).toBe(true);
    expect(a.endsWith("}\n")).toBe(true);
    expect(ok().text).toBe(a);
  });

  it("secrets(API キー・トークン)は設定に載せない", () => {
    const r = ok({ ...valid(), accountId: "" });
    expect(r.text).not.toContain("BOARD_API_KEY");
    expect(r.text).not.toContain("BOARD_API_TOKEN");
  });

  it("生成物に <...> のプレースホルダは残らない", () => {
    expect(findPlaceholderLines(ok().text)).toEqual([]);
  });
});

describe("検証エラー(項目名と理由だけ。値は載せない)", () => {
  const REQUIRED = [
    "workerName",
    "customDomain",
    "d1.databaseId",
    "d1.databaseName",
    "access.teamDomain",
    "access.aud",
  ];

  it.each(REQUIRED)("%s が無ければ missing", (name) => {
    expect(errorsOf(patched(name, undefined))).toEqual([{ name, reason: "missing" }]);
  });

  it("空文字・空白だけは未設定", () => {
    expect(errorsOf(patched("workerName", "  "))).toEqual([
      { name: "workerName", reason: "missing" },
    ]);
  });

  it("プレースホルダ(<...>)のままなら placeholder", () => {
    const raw = patched("access.aud", "<YOUR_ACCESS_AUD>");
    (raw.access as Record<string, unknown>).teamDomain = "<YOUR_TEAM>.cloudflareaccess.com";
    expect(errorsOf(raw)).toEqual([
      { name: "access.teamDomain", reason: "placeholder" },
      { name: "access.aud", reason: "placeholder" },
    ]);
  });

  it.each([
    ["workerName", "My_Board"],
    ["d1.databaseId", "not-a-uuid"],
    ["d1.databaseName", "has space"],
    ["access.teamDomain", "https://myteam.cloudflareaccess.com"],
    ["access.aud", "too-short"],
    ["customDomain", "https://board.example.org"],
    ["boardBaseUrl", "http://api.example.org"],
    ["boardBaseUrl", "https://api.example.org/v1"],
    ["boardUiBaseUrl", "http://ui.example.org"],
    ["boardUiBaseUrl", "ui.example.org"],
    ["accountId", "short"],
  ])("%s=%s は invalid", (name, value) => {
    const raw = valid();
    const [a, b] = name.split(".");
    if (b) (raw[a as string] as Record<string, unknown>)[b] = value;
    else raw[a as string] = value;
    expect(errorsOf(raw)).toEqual([{ name, reason: "invalid" }]);
  });

  it("文字列でない値は invalid", () => {
    expect(errorsOf(patched("workerName", 123))).toEqual([
      { name: "workerName", reason: "invalid" },
    ]);
  });

  it("未知の項目は unknown(タイプミスに気づける。項目名だけを出す)", () => {
    expect(errorsOf({ ...valid(), workername: "x" })).toEqual([
      { name: "workername", reason: "unknown" },
    ]);
    const raw = patched("d1", { ...(valid().d1 as object), extra: "x" });
    expect(errorsOf(raw)).toEqual([{ name: "d1.extra", reason: "unknown" }]);
  });

  it("オブジェクトでない設定・d1 / access がオブジェクトでないときは invalid", () => {
    expect(errorsOf([])).toEqual([{ name: "board.config.json", reason: "invalid" }]);
    expect(errorsOf("x")).toEqual([{ name: "board.config.json", reason: "invalid" }]);
    expect(errorsOf(patched("d1", "x"))).toEqual([{ name: "d1", reason: "invalid" }]);
    expect(errorsOf(patched("access", null))).toEqual([{ name: "access", reason: "invalid" }]);
  });

  it("formatErrors は項目名と理由だけを返す", () => {
    const lines = formatErrors(errorsOf(patched("d1.databaseId", "SECRET-LOOKING")));
    expect(lines).toHaveLength(1);
    expect(lines[0]).toContain("d1.databaseId");
    expect(lines.join("")).not.toContain("SECRET");
  });

  it("wrangler.jsonc が読めない・D1 の DB 束縛が無いなら wrangler.jsonc の invalid", () => {
    expect(renderWranglerConfig("{ broken", valid(), { relBase: ".." })).toEqual({
      ok: false,
      errors: [{ name: "wrangler.jsonc", reason: "invalid" }],
    });
    const noD1 = BASE.replace(/"d1_databases"[\s\S]*?\],\n/, "");
    expect(renderWranglerConfig(noD1, valid(), { relBase: ".." })).toEqual({
      ok: false,
      errors: [{ name: "wrangler.jsonc", reason: "invalid" }],
    });
  });

  it("設定のエラーは wrangler.jsonc のエラーより先に出る(両方は混ぜない)", () => {
    const r = renderWranglerConfig("{ broken", patched("workerName", undefined), { relBase: ".." });
    expect(r).toEqual({ ok: false, errors: [{ name: "workerName", reason: "missing" }] });
  });
});

describe("D1 の id が未作成のとき(setup の前提確認だけ)", () => {
  it("pendingD1 なら d1.databaseId のプレースホルダ・空・欠落を許し、ほかは検証する", () => {
    for (const v of ["<YOUR_D1_DATABASE_ID>", "", undefined]) {
      expect(validateConfig(patched("d1.databaseId", v), { pendingD1: true }).ok).toBe(true);
    }
    expect(errorsOf(patched("d1.databaseId", "not-a-uuid"), { pendingD1: true })).toEqual([
      { name: "d1.databaseId", reason: "invalid" },
    ]);
    const raw = patched("d1.databaseId", "<x>");
    (raw.access as Record<string, unknown>).aud = "<YOUR_ACCESS_AUD>";
    expect(errorsOf(raw, { pendingD1: true })).toEqual([
      { name: "access.aud", reason: "placeholder" },
    ]);
  });

  it("pendingD1 でも生成(render)は d1.databaseId が実値でないと失敗する", () => {
    const r = renderWranglerConfig(BASE, patched("d1.databaseId", "<x>"), { relBase: ".." });
    expect(r).toEqual({ ok: false, errors: [{ name: "d1.databaseId", reason: "placeholder" }] });
  });

  it("setD1Id は d1.databaseId だけを書き換え、ほかの項目と順序を保つ", () => {
    const text = `${JSON.stringify(patched("d1.databaseId", "<YOUR_D1_DATABASE_ID>"), null, 2)}\n`;
    const out = setD1Id(text, "11111111-2222-3333-4444-555555555555");
    const parsed = JSON.parse(out);
    expect(parsed.d1.databaseId).toBe("11111111-2222-3333-4444-555555555555");
    expect(parsed.d1.databaseName).toBe("my-board-cache");
    expect(Object.keys(parsed)).toEqual(Object.keys(valid()));
    expect(out.endsWith("\n")).toBe(true);
  });

  it("setD1Id は uuid でない値・読めない設定を拒否する", () => {
    expect(() => setD1Id("{}", "nope")).toThrow();
    expect(() => setD1Id("{ broken", "11111111-2222-3333-4444-555555555555")).toThrow();
  });

  it("findD1ByName は wrangler d1 list --json の出力から名前で uuid を引く", () => {
    const list = JSON.stringify([
      { uuid: "11111111-2222-3333-4444-555555555555", name: "other" },
      { uuid: "66666666-7777-8888-9999-000000000000", name: "my-board-cache" },
    ]);
    expect(findD1ByName(list, "my-board-cache")).toBe("66666666-7777-8888-9999-000000000000");
    expect(findD1ByName(list, "nothing")).toBeUndefined();
    expect(findD1ByName("not json", "x")).toBeUndefined();
  });
});

describe("example(deploy:check 用)", () => {
  const example = JSON.parse(exampleText);

  it("board.config.example.json は <YOUR_...> のプレースホルダで、そのままでは検証を通らない", () => {
    const errors = errorsOf(example);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.every((e) => e.reason === "placeholder")).toBe(true);
    expect(JSON.stringify(example)).not.toMatch(/[0-9a-f]{32}/);
  });

  it("exampleConfig はプレースホルダをダミー値に置き換え、生成できる", () => {
    const r = renderWranglerConfig(upstreamWrangler, exampleConfig(example), {
      relBase: "../..",
    });
    expect(r.ok).toBe(true);
  });

  it("ダミー値は実在しない値(ゼロ埋めと example ドメイン)", () => {
    expect(EXAMPLE_DUMMIES.databaseId).toBe("00000000-0000-0000-0000-000000000000");
    expect(EXAMPLE_DUMMIES.customDomain).toMatch(/example\./);
  });
});

describe("findPlaceholderLines(deploy が止まる条件)", () => {
  it("コメント行を除き、<...> が残る行番号を返す", () => {
    const text = '// <comment>\n{\n  "a": "<YOUR_X>",\n  "b": "ok"\n}\n';
    expect(findPlaceholderLines(text)).toEqual([3]);
  });
});

describe("healthzReport(setup の /healthz 確認)", () => {
  it("503 の本文から欠けている項目名・不正な項目名を返す", () => {
    const body = JSON.stringify({
      status: "misconfigured",
      missing: ["BOARD_API_KEY"],
      invalid: ["ACCESS_AUD"],
    });
    expect(healthzReport(body)).toEqual({ missing: ["BOARD_API_KEY"], invalid: ["ACCESS_AUD"] });
  });
  it("読めない本文は空", () => {
    expect(healthzReport("<html>")).toEqual({ missing: [], invalid: [] });
    expect(healthzReport(JSON.stringify({ missing: [1, "A"], invalid: "x" }))).toEqual({
      missing: ["A"],
      invalid: [],
    });
  });
});

describe("upstream の wrangler.jsonc(生成器の前提)", () => {
  it("生成器が相対パスを直せるキーだけを持つ(パスを持つキーを足したら core.ts を直す)", () => {
    const base = parseJsonc(upstreamWrangler) as Record<string, unknown>;
    const allowed = new Set([
      "$schema",
      "name",
      "main",
      "compatibility_date",
      "compatibility_flags",
      "workers_dev",
      "observability",
      "d1_databases",
      "vars",
    ]);
    expect(Object.keys(base).filter((k) => !allowed.has(k))).toEqual([]);
  });
});
