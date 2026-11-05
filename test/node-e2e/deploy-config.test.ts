// 生成スクリプトを実プロセスとして起動し、終了コードと生成ファイルを確かめる。
// 生成物が実際の `wrangler deploy --dry-run -c` を通ることも確認する(Cloudflare へは何も送らない)。
import { execFile } from "node:child_process";
import { cp, mkdir, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

const run = promisify(execFile);
const REPO = resolve(import.meta.dirname, "../..");
const MAIN = join(REPO, "scripts/deploy-config/main.ts");

// 実在しないダミー値。
const VALID = {
  workerName: "e2e-board",
  customDomain: "board.e2e.example.org",
  d1: { databaseId: "0123abcd-4567-89ab-cdef-0123456789ab", databaseName: "e2e-board-cache" },
  access: {
    teamDomain: "e2eteam.cloudflareaccess.com",
    aud: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // gitleaks:allow テスト用の合成値
  },
};

/** 親プロセスの Cloudflare の認証情報などを持ち込まない、最小の環境。 */
function cleanEnv(extra: Record<string, string> = {}): NodeJS.ProcessEnv {
  return { PATH: process.env.PATH, HOME: process.env.HOME, ...extra };
}

async function main(
  script: string,
  args: string[],
  cwd = REPO,
  input?: string,
): Promise<{ code: number; stdout: string; stderr: string }> {
  try {
    const p = run(process.execPath, [script, ...args], { cwd, env: cleanEnv() });
    if (input !== undefined) p.child.stdin?.end(input);
    const { stdout, stderr } = await p;
    return { code: 0, stdout, stderr };
  } catch (e) {
    const err = e as { code: number; stdout: string; stderr: string };
    return { code: err.code, stdout: err.stdout, stderr: err.stderr };
  }
}

let work: string;
beforeEach(async () => {
  work = await mkdtemp(join(tmpdir(), "deploy-config-"));
});
afterEach(async () => {
  await rm(work, { recursive: true, force: true });
});

async function writeConfig(value: unknown, name = "board.config.json"): Promise<string> {
  const p = join(work, name);
  await writeFile(p, `${JSON.stringify(value, null, 2)}\n`);
  return p;
}

/** 作業用の root に scripts・wrangler.jsonc・example を写す(実リポジトリを汚さない)。 */
async function copyRoot(): Promise<string> {
  const root = join(work, "root");
  await mkdir(join(root, "scripts"), { recursive: true });
  await cp(join(REPO, "scripts/deploy-config"), join(root, "scripts/deploy-config"), {
    recursive: true,
  });
  await cp(join(REPO, "wrangler.jsonc"), join(root, "wrangler.jsonc"));
  await cp(join(REPO, "board.config.example.json"), join(root, "board.config.example.json"));
  return root;
}

async function dryRun(configPath: string): Promise<string> {
  const { stdout, stderr } = await run(
    join(REPO, "node_modules/.bin/wrangler"),
    ["deploy", "--dry-run", "-c", configPath, "--outdir", join(work, "dist")],
    { cwd: REPO, env: cleanEnv({ CI: "1", WRANGLER_SEND_METRICS: "false" }) },
  );
  return stdout + stderr;
}

describe("deploy-config generate (実プロセス)", () => {
  it("設定ファイルから --dir に生成し、終了コード 0。wrangler.jsonc の設定を引き継ぐ", async () => {
    const cfg = await writeConfig({ ...VALID, boardUiBaseUrl: "https://ui.e2e.example.org" });
    const dir = join(work, "out");
    const r = await main(MAIN, ["generate", "--config", cfg, "--dir", dir]);
    expect(r.code).toBe(0);
    const text = await readFile(join(dir, "wrangler.jsonc"), "utf8");
    expect(text.startsWith("//")).toBe(true);
    const config = JSON.parse(text.replace(/^\/\/.*\n/, ""));
    expect(config.name).toBe("e2e-board");
    expect(config.d1_databases).toEqual([
      { binding: "DB", database_name: "e2e-board-cache", database_id: VALID.d1.databaseId },
    ]);
    expect(config.vars.ACCESS_TEAM_DOMAIN).toBe("e2eteam.cloudflareaccess.com");
    expect(config.vars.ACCESS_AUD).toBe(VALID.access.aud);
    expect(config.vars.BOARD_UI_BASE_URL).toBe("https://ui.e2e.example.org");
    expect(config.routes).toEqual([{ pattern: "board.e2e.example.org", custom_domain: true }]);
    expect(config.workers_dev).toBe(false);
    expect(config.env).toBeUndefined();
  });

  it("必須値が欠ければ終了コード 1、stderr に項目名、ファイルは作らない", async () => {
    const { d1: _omit, ...rest } = VALID;
    const cfg = await writeConfig({ ...rest, d1: { databaseName: "x" } });
    const dir = join(work, "out");
    const r = await main(MAIN, ["generate", "--config", cfg, "--dir", dir]);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("d1.databaseId");
    await expect(stat(join(dir, "wrangler.jsonc"))).rejects.toThrow();
  });

  it("形式不正・プレースホルダは終了コード 1 で、入力値を出力しない", async () => {
    const cfg = await writeConfig({
      ...VALID,
      d1: { ...VALID.d1, databaseId: "ZZZ-SECRET-LOOKING-VALUE" },
      access: { ...VALID.access, aud: "<aud>-SECRET-PLACEHOLDER" },
    });
    const dir = join(work, "out");
    const r = await main(MAIN, ["generate", "--config", cfg, "--dir", dir]);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("d1.databaseId");
    expect(r.stderr).toContain("access.aud");
    expect(r.stderr + r.stdout).not.toContain("SECRET");
    await expect(stat(join(dir, "wrangler.jsonc"))).rejects.toThrow();
  });

  it("設定ファイルが JSON でないとき、内容を出さずに終了コード 1", async () => {
    const p = join(work, "bad.json");
    await writeFile(p, "{ SECRET-NOT-JSON");
    const r = await main(MAIN, ["generate", "--config", p, "--dir", join(work, "out")]);
    expect(r.code).toBe(1);
    expect(r.stderr + r.stdout).not.toContain("SECRET");
  });

  it("設定ファイルが無ければ終了コード 1(example のコピーを案内)", async () => {
    const r = await main(MAIN, ["generate", "--config", join(work, "none.json")]);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("board.config.example.json");
  });

  it("不明な引数・サブコマンドは終了コード 2", async () => {
    expect((await main(MAIN, ["generate", "--nope"])).code).toBe(2);
    expect((await main(MAIN, ["nope"])).code).toBe(2);
    expect((await main(MAIN, [])).code).toBe(2);
  });

  it("既定の出力先はリポジトリ直下の deploy/、設定は board.config.json", async () => {
    const root = await copyRoot();
    await writeFile(join(root, "board.config.json"), JSON.stringify(VALID));
    const r = await main(join(root, "scripts/deploy-config/main.ts"), ["generate"], root);
    expect(r.code).toBe(0);
    const text = await readFile(join(root, "deploy/wrangler.jsonc"), "utf8");
    expect(text).toContain('"main": "../src/index.ts"');
  });
});

describe("deploy-config generate --example", () => {
  it("設定ファイルなしでダミー値の設定を .wrangler/deploy-example に生成する(deploy/ は作らない)", async () => {
    const root = await copyRoot();
    const r = await main(
      join(root, "scripts/deploy-config/main.ts"),
      ["generate", "--example"],
      root,
    );
    expect(r.code).toBe(0);
    const text = await readFile(join(root, ".wrangler/deploy-example/wrangler.jsonc"), "utf8");
    expect(text).toContain('"main": "../../src/index.ts"');
    expect(text).toContain("00000000-0000-0000-0000-000000000000");
    await expect(stat(join(root, "deploy"))).rejects.toThrow();
  });

  it("生成物が wrangler deploy --dry-run -c を通る", async () => {
    const r = await main(MAIN, ["generate", "--example"]);
    expect(r.code).toBe(0);
    const out = await dryRun(join(REPO, ".wrangler/deploy-example/wrangler.jsonc"));
    expect(out).toContain("board-example");
    expect(out).toContain("--dry-run");
  }, 120_000);
});

describe("deploy-config の通し(生成 → dry-run → check)", () => {
  it("生成物が wrangler deploy --dry-run -c を通り、check が一致・不一致を判定する", async () => {
    // main の相対パス(../../src/index.ts)が解決されるよう、生成物はリポジトリ内の .wrangler/ に置く(gitignore 済み)。
    const dir = join(REPO, ".wrangler", `e2e-deploy-${process.pid}`);
    const cfg = await writeConfig({ ...VALID, boardUiBaseUrl: "https://ui.e2e.example.org" });
    const args = ["--config", cfg, "--dir", dir];
    try {
      expect((await main(MAIN, ["generate", ...args])).code).toBe(0);
      const out = await dryRun(join(dir, "wrangler.jsonc"));
      expect(out).toContain("e2e-board");
      expect(out).toContain("--dry-run");

      // 一致
      const ok = await main(MAIN, ["check", ...args]);
      expect(ok.code).toBe(0);

      // 設定を直したのに生成し直していない
      await writeConfig({ ...VALID, workerName: "e2e-board-renamed" });
      const stale = await main(MAIN, ["check", ...args]);
      expect(stale.code).toBe(1);
      expect(stale.stderr).toContain("差分あり");
      await writeConfig({ ...VALID, boardUiBaseUrl: "https://ui.e2e.example.org" });
      expect((await main(MAIN, ["check", ...args])).code).toBe(0);

      // 生成物を手で書き換えた
      const gen = join(dir, "wrangler.jsonc");
      const original = await readFile(gen, "utf8");
      await writeFile(gen, original.replace('"workers_dev": false', '"workers_dev": true'));
      expect((await main(MAIN, ["check", ...args])).code).toBe(1);
      await writeFile(gen, original);

      // 生成物が無い
      await rm(gen);
      const missing = await main(MAIN, ["check", ...args]);
      expect(missing.code).toBe(1);
      expect(missing.stderr).toContain("無い");
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  }, 120_000);

  it("upstream の wrangler.jsonc が変わったのに取り込んでいないと check が失敗する", async () => {
    const root = await copyRoot();
    const script = join(root, "scripts/deploy-config/main.ts");
    const cfg = await writeConfig(VALID);
    const args = ["--config", cfg, "--dir", join(root, "deploy")];
    expect((await main(script, ["generate", ...args], root)).code).toBe(0);
    expect((await main(script, ["check", ...args], root)).code).toBe(0);
    const w = join(root, "wrangler.jsonc");
    await writeFile(w, (await readFile(w, "utf8")).replace("2026-10-13", "2026-10-14"));
    const r = await main(script, ["check", ...args], root);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("upstream");
  });

  it("check は --example を受け付けない", async () => {
    expect((await main(MAIN, ["check", "--example"])).code).toBe(2);
  });
});

describe("deploy-config の補助サブコマンド(setup が使う)", () => {
  it("validate: 実値なら 0、プレースホルダは 1(--pending-d1 なら d1 の id だけ許す)", async () => {
    const ok = await writeConfig(VALID);
    expect((await main(MAIN, ["validate", "--config", ok])).code).toBe(0);
    const pending = await writeConfig(
      { ...VALID, d1: { ...VALID.d1, databaseId: "<YOUR_D1_DATABASE_ID>" } },
      "pending.json",
    );
    expect((await main(MAIN, ["validate", "--config", pending])).code).toBe(1);
    expect((await main(MAIN, ["validate", "--pending-d1", "--config", pending])).code).toBe(0);
    const example = join(REPO, "board.config.example.json");
    const r = await main(MAIN, ["validate", "--pending-d1", "--config", example]);
    expect(r.code).toBe(1);
    expect(r.stderr).toContain("workerName");
    expect(r.stderr).not.toContain("d1.databaseId");
  });

  it("get: 設定値を 1 つ出す。d1.databaseId が未作成なら空行", async () => {
    const cfg = await writeConfig(VALID);
    expect((await main(MAIN, ["get", "customDomain", "--config", cfg])).stdout.trim()).toBe(
      "board.e2e.example.org",
    );
    const pending = await writeConfig({ ...VALID, d1: { databaseName: "x" } }, "pending.json");
    const r = await main(MAIN, ["get", "d1.databaseId", "--config", pending]);
    expect(r.code).toBe(0);
    expect(r.stdout.trim()).toBe("");
    expect((await main(MAIN, ["get", "nope", "--config", cfg])).code).toBe(2);
  });

  it("set-d1: databaseId だけを書き戻す。uuid でなければ書かない", async () => {
    const cfg = await writeConfig({
      ...VALID,
      d1: { ...VALID.d1, databaseId: "<YOUR_D1_DATABASE_ID>" },
    });
    const id = "11111111-2222-3333-4444-555555555555";
    expect((await main(MAIN, ["set-d1", id, "--config", cfg])).code).toBe(0);
    const written = JSON.parse(await readFile(cfg, "utf8"));
    expect(written.d1).toEqual({ databaseId: id, databaseName: "e2e-board-cache" });
    expect(written.workerName).toBe("e2e-board");
    expect((await main(MAIN, ["set-d1", "nope", "--config", cfg])).code).toBe(1);
    expect(JSON.parse(await readFile(cfg, "utf8")).d1.databaseId).toBe(id);
  });

  it("find-d1 / healthz-report: 標準入力を読む", async () => {
    const list = JSON.stringify([{ uuid: "11111111-2222-3333-4444-555555555555", name: "a" }]);
    const found = await main(MAIN, ["find-d1", "a"], REPO, list);
    expect(found.stdout.trim()).toBe("11111111-2222-3333-4444-555555555555");
    expect((await main(MAIN, ["find-d1", "b"], REPO, list)).code).toBe(1);
    const rep = await main(
      MAIN,
      ["healthz-report"],
      REPO,
      JSON.stringify({ missing: ["BOARD_API_KEY"], invalid: ["ACCESS_AUD"] }),
    );
    expect(rep.stdout).toContain("BOARD_API_KEY");
    expect(rep.stdout).toContain("ACCESS_AUD");
  });
});
