// 生成スクリプトを実プロセスとして起動し、終了コードと生成ファイルを確かめる。
// 生成物が実際の `wrangler deploy --dry-run -c` を通ることも確認する(Cloudflare へは何も送らない)。
import { execFile } from "node:child_process";
import { cp, mkdir, mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

const run = promisify(execFile);
const REPO = resolve(import.meta.dirname, "../..");
const SCRIPT_DIR = join(REPO, "scripts/render-wrangler-config");

// 実在しないダミー値。
const VALID_ENV = {
  BOARD_WORKER_NAME: "e2e-board",
  BOARD_D1_DATABASE_ID: "0123abcd-4567-89ab-cdef-0123456789ab",
  BOARD_D1_DATABASE_NAME: "e2e-board-cache",
  BOARD_ACCESS_TEAM_DOMAIN: "e2eteam.cloudflareaccess.com",
  BOARD_ACCESS_AUD: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // gitleaks:allow テスト用の合成値
  BOARD_CUSTOM_DOMAIN: "board.e2e.example.org",
};

/** 親プロセスの BOARD_* や Cloudflare の認証情報を持ち込まない、最小の環境。 */
function cleanEnv(extra: Record<string, string> = {}): NodeJS.ProcessEnv {
  return { PATH: process.env.PATH, HOME: process.env.HOME, ...extra };
}

async function render(
  script: string,
  args: string[],
  env: Record<string, string>,
  cwd = REPO,
): Promise<{ code: number; stdout: string; stderr: string }> {
  try {
    const { stdout, stderr } = await run(process.execPath, [script, ...args], {
      cwd,
      env: cleanEnv(env),
    });
    return { code: 0, stdout, stderr };
  } catch (e) {
    const err = e as { code: number; stdout: string; stderr: string };
    return { code: err.code, stdout: err.stdout, stderr: err.stderr };
  }
}

const MAIN = join(SCRIPT_DIR, "main.ts");
let work: string;

beforeEach(async () => {
  work = await mkdtemp(join(tmpdir(), "render-wrangler-"));
});
afterEach(async () => {
  await rm(work, { recursive: true, force: true });
});

/** 作業用の root に scripts と wrangler.jsonc を写す(実リポジトリを汚さない)。 */
async function copyRoot(): Promise<string> {
  const root = join(work, "root");
  await mkdir(join(root, "scripts"), { recursive: true });
  await cp(SCRIPT_DIR, join(root, "scripts/render-wrangler-config"), { recursive: true });
  await cp(join(REPO, "wrangler.jsonc"), join(root, "wrangler.jsonc"));
  return root;
}

describe("render-wrangler-config (実プロセス)", () => {
  it("環境変数から --out に生成し、終了コード 0。wrangler.jsonc の設定を引き継ぐ", async () => {
    const out = join(work, "out.jsonc");
    const r = await render(MAIN, ["--out", out], {
      ...VALID_ENV,
      BOARD_UI_BASE_URL: "https://ui.e2e.example.org",
    });
    expect(r.code).toBe(0);
    const config = JSON.parse(await readFile(out, "utf8"));
    expect(config.name).toBe("e2e-board");
    expect(config.d1_databases).toEqual([
      {
        binding: "DB",
        database_name: "e2e-board-cache",
        database_id: VALID_ENV.BOARD_D1_DATABASE_ID,
      },
    ]);
    expect(config.vars.ACCESS_TEAM_DOMAIN).toBe("e2eteam.cloudflareaccess.com");
    expect(config.vars.ACCESS_AUD).toBe(VALID_ENV.BOARD_ACCESS_AUD);
    expect(config.vars.BOARD_UI_BASE_URL).toBe("https://ui.e2e.example.org");
    expect(config.routes).toEqual([{ pattern: "board.e2e.example.org", custom_domain: true }]);
    expect(config.workers_dev).toBe(false);
    expect(config.env).toBeUndefined();
    const upstream = JSON.parse(
      (await readFile(join(REPO, "wrangler.jsonc"), "utf8"))
        .replace(/^\s*\/\/.*$/gm, "")
        .replace(/,(\s*[}\]])/g, "$1"),
    );
    expect(config.compatibility_date).toBe(upstream.compatibility_date);
    expect(config.main).toBe(upstream.main);
  });

  it("必須値が欠ければ終了コード 1、stderr に項目名、ファイルは作らない", async () => {
    const out = join(work, "out.jsonc");
    const { BOARD_D1_DATABASE_ID: _omit, ...rest } = VALID_ENV;
    const r = await render(MAIN, ["--out", out], rest);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("BOARD_D1_DATABASE_ID");
    await expect(stat(out)).rejects.toThrow();
  });

  it("形式不正・プレースホルダは終了コード 1 で、入力値を出力しない", async () => {
    const out = join(work, "out.jsonc");
    const r = await render(MAIN, ["--out", out], {
      ...VALID_ENV,
      BOARD_D1_DATABASE_ID: "ZZZ-SECRET-LOOKING-VALUE",
      BOARD_ACCESS_AUD: "<aud>-SECRET-PLACEHOLDER",
    });
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("BOARD_D1_DATABASE_ID");
    expect(r.stderr).toContain("BOARD_ACCESS_AUD");
    expect(r.stderr + r.stdout).not.toContain("SECRET");
    await expect(stat(out)).rejects.toThrow();
  });

  it("--example は環境変数なしでダミー値の設定を生成する(既定の出力先は実用の生成物と別)", async () => {
    const root = await copyRoot();
    const r = await render(
      join(root, "scripts/render-wrangler-config/main.ts"),
      ["--example"],
      {},
      root,
    );
    expect(r.code).toBe(0);
    const config = JSON.parse(await readFile(join(root, "wrangler.deploy.example.jsonc"), "utf8"));
    expect(config.name).toBeTruthy();
    expect(config.d1_databases[0].database_id).toMatch(/^[0-9a-f-]{36}$/);
    await expect(stat(join(root, "wrangler.deploy.jsonc"))).rejects.toThrow();
  });

  it("--out 省略時はリポジトリ直下の wrangler.deploy.jsonc に書く", async () => {
    const root = await copyRoot();
    const r = await render(
      join(root, "scripts/render-wrangler-config/main.ts"),
      [],
      VALID_ENV,
      root,
    );
    expect(r.code).toBe(0);
    const config = JSON.parse(await readFile(join(root, "wrangler.deploy.jsonc"), "utf8"));
    expect(config.name).toBe("e2e-board");
  });

  it("不明な引数は終了コード 2", async () => {
    const r = await render(MAIN, ["--nope"], VALID_ENV);
    expect(r.code).toBe(2);
  });

  it("生成物が wrangler deploy --dry-run -c を通る(BOARD_UI_BASE_URL あり / なし)", async () => {
    // main の相対パスが解決されるよう、生成物はリポジトリ直下に置く(wrangler.deploy*.jsonc は gitignore 済み)。
    const variants: Record<string, string>[] = [
      {},
      { BOARD_UI_BASE_URL: "https://ui.e2e.example.org" },
    ];
    for (const extra of variants) {
      const out = join(REPO, `wrangler.deploy.e2e-${process.pid}.jsonc`);
      try {
        const r = await render(MAIN, ["--out", out], { ...VALID_ENV, ...extra });
        expect(r.code).toBe(0);
        const { stdout, stderr } = await run(
          join(REPO, "node_modules/.bin/wrangler"),
          ["deploy", "--dry-run", "-c", out, "--outdir", join(work, "dist")],
          { cwd: REPO, env: cleanEnv({ CI: "1", WRANGLER_SEND_METRICS: "false" }) },
        );
        expect(stdout + stderr).toContain("e2e-board");
        expect(stdout + stderr).toContain("--dry-run");
      } finally {
        await rm(out, { force: true });
      }
    }
  }, 120_000);
});
